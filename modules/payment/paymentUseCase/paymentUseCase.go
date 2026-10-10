package paymentUsecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"math"
	"sync"
	"time"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/inventory"
	"github.com/watcharaphong99/InwzaShop/modules/item"
	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
	"github.com/watcharaphong99/InwzaShop/modules/payment/paymentRepository"
	"github.com/watcharaphong99/InwzaShop/modules/player"
)

const (
	paymentStepTimeout = 8 * time.Second
	// orderWorkflowStaleAfter ระยะที่ถือว่า ProcessInstance ค้าง
	orderWorkflowStaleAfter = 45 * time.Second
	// OrderWorkflowRecoverInterval ช่วงรัน RecoverStaleOrderWorkflows ซ้ำ
	OrderWorkflowRecoverInterval = 1 * time.Minute

	workflowRunning   = "running"
	workflowCompleted = "completed"
	workflowFailed    = "failed"
	stepSent          = "sent"
	stepDone          = "done"

	stepCapturePayment  = "capture_payment"   // เดิม dock_money
	stepFulfillLineItem = "fulfill_line_item" // เดิม add_item
	stepAddMoney        = "add_money"
	stepRemoveItem      = "remove_item"

	// legacy step kinds ใน Mongo ก่อน rename
	stepCapturePaymentLegacy  = "dock_money"
	stepFulfillLineItemLegacy = "add_item"
)

type (
	PaymentUsecaseService interface {
		GetOffset(pctx context.Context) (int64, error)
		UpserOffset(pctx context.Context, offset int64) error
		ResolvePricingFromCatalog(pctx context.Context, grpcUrl string, req []*payment.ItemServiceReqDatum) error
		AcceptPaymentReply(res *payment.PaymentTransferRes)
		RecoverStaleOrderWorkflows(pctx context.Context, cfg *config.Config)
		ExecutePurchase(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) ([]*payment.PaymentTransferRes, error)
		SellItem(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) ([]*payment.PaymentTransferRes, error)
	}

	paymentUsecase struct {
		paymentRepository paymentRepository.PaymentRepositoryService
		mu                sync.Mutex
		waiters           map[string]chan *payment.PaymentTransferRes
	}

	paymentStep struct {
		EventID       string
		ItemID        string
		Amount        float64
		TransactionID string
		InventoryID   string
	}
)

func NewPaymentUsecase(paymentRepository paymentRepository.PaymentRepositoryService) PaymentUsecaseService {
	return &paymentUsecase{
		paymentRepository: paymentRepository,
		waiters:           make(map[string]chan *payment.PaymentTransferRes),
	}
}

func (u *paymentUsecase) GetOffset(pctx context.Context) (int64, error) {
	return u.paymentRepository.GetOffset(pctx)
}
func (u *paymentUsecase) UpserOffset(pctx context.Context, offset int64) error {
	return u.paymentRepository.UpsertOffset(pctx, offset)
}

func (u *paymentUsecase) AcceptPaymentReply(res *payment.PaymentTransferRes) {
	if res == nil || res.EventId == "" {
		log.Printf("Error: payment reply missing event_id")
		return
	}

	u.mu.Lock()
	ch := u.waiters[res.EventId]
	u.mu.Unlock()
	if ch == nil {
		log.Printf("Info: payment reply event_id=%s has no waiter", res.EventId)
		return
	}

	select {
	case ch <- res:
	default:
		log.Printf("Error: payment reply event_id=%s dropped", res.EventId)
	}
}

func (u *paymentUsecase) register(eventID string) <-chan *payment.PaymentTransferRes {
	ch := make(chan *payment.PaymentTransferRes, 1)
	u.mu.Lock()
	u.waiters[eventID] = ch
	u.mu.Unlock()
	return ch
}

func (u *paymentUsecase) unregister(eventID string) {
	u.mu.Lock()
	delete(u.waiters, eventID)
	u.mu.Unlock()
}

func (u *paymentUsecase) waitReply(pctx context.Context, eventID string, ch <-chan *payment.PaymentTransferRes) (*payment.PaymentTransferRes, error) {
	timer := time.NewTimer(paymentStepTimeout)
	defer timer.Stop()

	select {
	case <-pctx.Done():
		return nil, errors.New("error: payment step cancelled")
	case <-timer.C:
		return nil, errors.New("error: payment step timeout")
	case res := <-ch:
		if res == nil || res.EventId != eventID {
			return nil, errors.New("error: payment reply mismatch")
		}
		return res, nil
	}
}

// ExecutePurchase ดำเนินการซื้อ (เดิม BuyItem): CapturePayment แล้ว FulfillLineItem
func (u *paymentUsecase) ExecutePurchase(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) ([]*payment.PaymentTransferRes, error) {
	if err := u.prepareItems(pctx, cfg, req); err != nil {
		return nil, err
	}

	workflow := &payment.OrderWorkflow{ID: newEventID(), PlayerID: playerId, Action: "buy", Status: workflowRunning}
	if err := u.saveOrderWorkflow(workflow); err != nil {
		return nil, errors.New("error: buy item failed")
	}

	for _, item := range req.Items {
		step := paymentStep{EventID: newEventID(), ItemID: item.ItemId, Amount: item.Price}
		if err := u.trackStep(workflow, step, stepCapturePayment, stepSent); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		res, err := u.capturePayment(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("capture payment", step.EventID, res, err)
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		if err := u.finishStep(workflow, res.TransactionId, ""); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
	}

	out := make([]*payment.PaymentTransferRes, 0, len(req.Items))
	for _, paid := range doneSteps(workflow, stepCapturePayment) {
		step := paymentStep{EventID: newEventID(), ItemID: paid.ItemID, Amount: paid.Amount, TransactionID: paid.TransactionID}
		if err := u.trackStep(workflow, step, stepFulfillLineItem, stepSent); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		res, err := u.fulfillLineItem(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("fulfill line item", step.EventID, res, err)
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		if err := u.finishStep(workflow, paid.TransactionID, res.InventoryId); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		out = append(out, &payment.PaymentTransferRes{
			EventId:       step.EventID,
			InventoryId:   res.InventoryId,
			TransactionId: paid.TransactionID,
			PlayerId:      playerId,
			ItemId:        paid.ItemID,
			Amount:        paid.Amount,
		})
	}

	workflow.Status = workflowCompleted
	if err := u.saveOrderWorkflow(workflow); err != nil {
		return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
	}
	return out, nil
}

func (u *paymentUsecase) SellItem(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) ([]*payment.PaymentTransferRes, error) {
	if err := u.prepareItems(pctx, cfg, req); err != nil {
		return nil, err
	}

	workflow := &payment.OrderWorkflow{ID: newEventID(), PlayerID: playerId, Action: "sell", Status: workflowRunning}
	if err := u.saveOrderWorkflow(workflow); err != nil {
		return nil, errors.New("error: sell item failed")
	}

	for _, item := range req.Items {
		step := paymentStep{EventID: newEventID(), ItemID: item.ItemId, Amount: item.Price}
		if err := u.trackStep(workflow, step, stepRemoveItem, stepSent); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		res, err := u.removeItem(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("sell remove item", step.EventID, res, err)
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		if err := u.finishStep(workflow, "", ""); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
	}

	out := make([]*payment.PaymentTransferRes, 0, len(req.Items))
	for _, item := range doneSteps(workflow, stepRemoveItem) {
		payout := sellPayout(item.Amount)
		if payout <= 0 {
			log.Printf("Error: sell payout is zero item_id=%s", item.ItemID)
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		step := paymentStep{EventID: newEventID(), ItemID: item.ItemID, Amount: payout}
		if err := u.trackStep(workflow, step, stepAddMoney, stepSent); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		res, err := u.addMoney(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("sell add money", step.EventID, res, err)
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		if err := u.finishStep(workflow, res.TransactionId, ""); err != nil {
			return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
		}
		out = append(out, &payment.PaymentTransferRes{
			EventId:       step.EventID,
			TransactionId: res.TransactionId,
			PlayerId:      playerId,
			ItemId:        item.ItemID,
			Amount:        item.Amount,
		})
	}

	workflow.Status = workflowCompleted
	if err := u.saveOrderWorkflow(workflow); err != nil {
		return nil, u.abortOrderWorkflow(pctx, cfg, workflow)
	}
	return out, nil
}

// RecoverStaleOrderWorkflows ชดเชย ProcessInstance ที่ค้าง running
func (u *paymentUsecase) RecoverStaleOrderWorkflows(pctx context.Context, cfg *config.Config) {
	workflows, err := u.paymentRepository.ListStaleOrderWorkflows(pctx, time.Now().Add(-orderWorkflowStaleAfter))
	if err != nil {
		log.Printf("Error: recover order workflow: %s", err.Error())
		return
	}
	for _, workflow := range workflows {
		log.Printf("Info: recover stale order workflow %s action=%s", workflow.ID, workflow.Action)
		u.compensate(pctx, cfg, workflow)
		workflow.Status = workflowFailed
		if err := u.saveOrderWorkflow(workflow); err != nil {
			log.Printf("Error: mark recovered workflow %s: %s", workflow.ID, err.Error())
		}
	}
}

func (u *paymentUsecase) trackStep(workflow *payment.OrderWorkflow, step paymentStep, kind, status string) error {
	workflow.Steps = append(workflow.Steps, payment.OrderWorkflowStep{
		EventID:       step.EventID,
		Kind:          kind,
		ItemID:        step.ItemID,
		Amount:        step.Amount,
		TransactionID: step.TransactionID,
		InventoryID:   step.InventoryID,
		Status:        status,
	})
	return u.saveOrderWorkflow(workflow)
}

func (u *paymentUsecase) finishStep(workflow *payment.OrderWorkflow, transactionID, inventoryID string) error {
	last := &workflow.Steps[len(workflow.Steps)-1]
	last.Status = stepDone
	if transactionID != "" {
		last.TransactionID = transactionID
	}
	if inventoryID != "" {
		last.InventoryID = inventoryID
	}
	return u.saveOrderWorkflow(workflow)
}

func (u *paymentUsecase) abortOrderWorkflow(pctx context.Context, cfg *config.Config, workflow *payment.OrderWorkflow) error {
	u.compensate(pctx, cfg, workflow)
	workflow.Status = workflowFailed
	if err := u.saveOrderWorkflow(workflow); err != nil {
		log.Printf("Error: abort order workflow %s: %s", workflow.ID, err.Error())
	}
	if workflow.Action == "sell" {
		return errors.New("error: sell item failed")
	}
	return errors.New("error: buy item failed")
}

func (u *paymentUsecase) compensate(pctx context.Context, cfg *config.Config, workflow *payment.OrderWorkflow) {
	money := make([]paymentStep, 0)
	added := make([]paymentStep, 0)
	removed := make([]paymentStep, 0)
	for _, step := range workflow.Steps {
		if step.Status != stepDone {
			continue
		}
		recorded := paymentStep{
			EventID:       step.EventID,
			ItemID:        step.ItemID,
			Amount:        step.Amount,
			TransactionID: step.TransactionID,
			InventoryID:   step.InventoryID,
		}
		switch step.Kind {
		case stepCapturePayment, stepCapturePaymentLegacy, stepAddMoney:
			money = append(money, recorded)
		case stepFulfillLineItem, stepFulfillLineItemLegacy:
			added = append(added, recorded)
		case stepRemoveItem:
			removed = append(removed, recorded)
		}
	}
	u.rollbackAddedItems(pctx, cfg, added)
	u.rollbackRemovedItems(pctx, cfg, workflow.PlayerID, removed)
	u.rollbackMoney(pctx, cfg, money)
}

func (u *paymentUsecase) saveOrderWorkflow(workflow *payment.OrderWorkflow) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := u.paymentRepository.SaveOrderWorkflow(ctx, workflow); err != nil {
		log.Printf("Error: save order workflow %s: %s", workflow.ID, err.Error())
		return err
	}
	return nil
}

func stepKindMatches(storedKind, targetKind string) bool {
	if storedKind == targetKind {
		return true
	}
	switch targetKind {
	case stepCapturePayment:
		return storedKind == stepCapturePaymentLegacy
	case stepFulfillLineItem:
		return storedKind == stepFulfillLineItemLegacy
	default:
		return false
	}
}

func doneSteps(workflow *payment.OrderWorkflow, kind string) []paymentStep {
	steps := make([]paymentStep, 0)
	for _, step := range workflow.Steps {
		if !stepKindMatches(step.Kind, kind) || step.Status != stepDone {
			continue
		}
		steps = append(steps, paymentStep{
			EventID:       step.EventID,
			ItemID:        step.ItemID,
			Amount:        step.Amount,
			TransactionID: step.TransactionID,
			InventoryID:   step.InventoryID,
		})
	}
	return steps
}

func (u *paymentUsecase) prepareItems(pctx context.Context, cfg *config.Config, req *payment.ItemServiceReq) error {
	if req == nil || len(req.Items) == 0 {
		return errors.New("error: items is empty")
	}
	if err := u.ResolvePricingFromCatalog(pctx, cfg.Grpc.ItemUrl, req.Items); err != nil {
		return err
	}
	for _, item := range req.Items {
		if item.Price <= 0 {
			return errors.New("error: item price is invalid")
		}
	}
	return nil
}

// capturePayment หักเงินผ่าน Kafka (เดิม dockMoney)
func (u *paymentUsecase) capturePayment(pctx context.Context, cfg *config.Config, playerId string, step paymentStep) (*payment.PaymentTransferRes, error) {
	ch := u.register(step.EventID)
	defer u.unregister(step.EventID)

	if err := u.paymentRepository.CapturePayment(pctx, cfg, &player.CreatePlayerTransactionReq{
		PlayerId: playerId,
		Amount:   -step.Amount,
		EventId:  step.EventID,
	}); err != nil {
		return nil, err
	}
	return u.waitReply(pctx, step.EventID, ch)
}

func (u *paymentUsecase) addMoney(pctx context.Context, cfg *config.Config, playerId string, step paymentStep) (*payment.PaymentTransferRes, error) {
	ch := u.register(step.EventID)
	defer u.unregister(step.EventID)

	if err := u.paymentRepository.AddPlayerMoney(pctx, cfg, &player.CreatePlayerTransactionReq{
		PlayerId: playerId,
		Amount:   step.Amount,
		EventId:  step.EventID,
	}); err != nil {
		return nil, err
	}
	return u.waitReply(pctx, step.EventID, ch)
}

// fulfillLineItem ใส่ของ / GrantEntitlement (เดิม addItem)
func (u *paymentUsecase) fulfillLineItem(pctx context.Context, cfg *config.Config, playerId string, step paymentStep) (*payment.PaymentTransferRes, error) {
	ch := u.register(step.EventID)
	defer u.unregister(step.EventID)

	if err := u.paymentRepository.AddPlayerItem(pctx, cfg, &inventory.UpdateInventoryReq{
		PlayerId: playerId,
		ItemId:   step.ItemID,
		EventId:  step.EventID,
	}); err != nil {
		return nil, err
	}
	return u.waitReply(pctx, step.EventID, ch)
}

func (u *paymentUsecase) removeItem(pctx context.Context, cfg *config.Config, playerId string, step paymentStep) (*payment.PaymentTransferRes, error) {
	ch := u.register(step.EventID)
	defer u.unregister(step.EventID)

	if err := u.paymentRepository.RemovePlayerItem(pctx, cfg, &inventory.UpdateInventoryReq{
		PlayerId: playerId,
		ItemId:   step.ItemID,
		EventId:  step.EventID,
	}); err != nil {
		return nil, err
	}
	return u.waitReply(pctx, step.EventID, ch)
}

func (u *paymentUsecase) rollbackMoney(pctx context.Context, cfg *config.Config, steps []paymentStep) {
	for _, step := range steps {
		if err := u.paymentRepository.RollbackTransaction(pctx, cfg, &player.RollbackPlayerTransactionReq{
			TransactionId: step.TransactionID,
			EventId:       step.EventID,
		}); err != nil {
			log.Printf("Error: rollback money event_id=%s: %s", step.EventID, err.Error())
		}
	}
}

func (u *paymentUsecase) rollbackAddedItems(pctx context.Context, cfg *config.Config, steps []paymentStep) {
	for _, step := range steps {
		if err := u.paymentRepository.RollbackAddPlayerItem(pctx, cfg, &inventory.RollbackPlayerInventoryReq{
			InventoryId: step.InventoryID,
			EventId:     step.EventID,
		}); err != nil {
			log.Printf("Error: rollback add item event_id=%s: %s", step.EventID, err.Error())
		}
	}
}

func (u *paymentUsecase) rollbackRemovedItems(pctx context.Context, cfg *config.Config, playerId string, steps []paymentStep) {
	for _, step := range steps {
		if err := u.paymentRepository.RollbackRemovePlayerItem(pctx, cfg, &inventory.RollbackPlayerInventoryReq{
			PlayerId: playerId,
			ItemId:   step.ItemID,
			EventId:  step.EventID,
		}); err != nil {
			log.Printf("Error: rollback remove item event_id=%s: %s", step.EventID, err.Error())
		}
	}
}

func (u *paymentUsecase) logStep(step, eventID string, res *payment.PaymentTransferRes, err error) {
	if err != nil {
		log.Printf("Error: %s event_id=%s: %s", step, eventID, err.Error())
		return
	}
	if res != nil && res.Error != "" {
		log.Printf("Error: %s event_id=%s: %s", step, eventID, res.Error)
	}
}

// ResolvePricingFromCatalog ดึงราคาจาก ProductCatalog (เดิม FindItemsInIds)
func (u *paymentUsecase) ResolvePricingFromCatalog(pctx context.Context, grpcUrl string, req []*payment.ItemServiceReqDatum) error {
	setIds := make(map[string]bool)
	for _, v := range req {
		if !setIds[v.ItemId] {
			setIds[v.ItemId] = true
		}
	}

	itemData, err := u.paymentRepository.FindProductsInCatalog(pctx, grpcUrl, &itemPb.FindItemsInIdsReq{
		Ids: func() []string {
			itemIds := make([]string, 0)
			for k := range setIds {
				itemIds = append(itemIds, k)
			}
			return itemIds
		}(),
	})
	if err != nil {
		log.Printf("Error: ResolvePricingFromCatalog failed: %s", err.Error())
		return errors.New("error: items not found")
	}

	itemMaps := make(map[string]*item.ItemShowCase)
	for _, v := range itemData.Items {
		itemMaps[v.Id] = &item.ItemShowCase{
			ItemId:   v.Id,
			Title:    v.Title,
			Price:    v.Price,
			ImageUrl: v.ImageUrl,
			Damage:   int(v.Damage),
		}
	}

	for i := range req {
		found, ok := itemMaps[req[i].ItemId]
		if !ok {
			log.Printf("Error: ResolvePricingFromCatalog: item %s not found", req[i].ItemId)
			return errors.New("error: items not found")
		}
		req[i].Price = found.Price
	}

	return nil
}

func newEventID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		log.Printf("Error: new event id: %s", err.Error())
	}
	return hex.EncodeToString(buf)
}

func sellPayout(price float64) float64 {
	return math.Round(price*50) / 100
}
