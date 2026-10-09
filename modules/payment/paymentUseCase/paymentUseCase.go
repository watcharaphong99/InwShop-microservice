package paymentUsecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	paymentStepTimeout  = 8 * time.Second
	sagaStaleAfter      = 45 * time.Second
	// SagaRecoverInterval ช่วงรัน RecoverStaleSagas ซ้ำ (stale saga loop)
	SagaRecoverInterval = 1 * time.Minute

	sagaRunning   = "running"
	sagaCompleted = "completed"
	sagaFailed    = "failed"
	stepSent      = "sent"
	stepDone      = "done"

	stepDockMoney  = "dock_money"
	stepAddMoney   = "add_money"
	stepAddItem    = "add_item"
	stepRemoveItem = "remove_item"
)

type (
	PaymentUsecaseService interface {
		GetOffset(pctx context.Context) (int64, error)
		UpserOffset(pctx context.Context, offset int64) error
		FindItemsInIds(pctx context.Context, grpcUrl string, req []*payment.ItemServiceReqDatum) error
		AcceptPaymentReply(res *payment.PaymentTransferRes)
		RecoverStaleSagas(pctx context.Context, cfg *config.Config)
		BuyItem(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) ([]*payment.PaymentTransferRes, error)
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

func (u *paymentUsecase) BuyItem(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) ([]*payment.PaymentTransferRes, error) {
	// หาราคาของ ItemId
	if err := u.prepareItems(pctx, cfg, req); err != nil {
		return nil, err
	}

	//สร้าง newEventId เพื่อ save ใน saga
	saga := &payment.Saga{ID: newEventID(), PlayerID: playerId, Action: "buy", Status: sagaRunning}
	fmt.Println("print--sage", saga)
	if err := u.saveSaga(saga); err != nil {
		return nil, errors.New("error: buy item failed")
	}

	//ตัดเงิน ส่ง message เข้า kafka
	for _, item := range req.Items {
		// สร้าง step เพื่อเอาใว้ track
		step := paymentStep{EventID: newEventID(), ItemID: item.ItemId, Amount: item.Price}
		if err := u.trackStep(saga, step, stepDockMoney, stepSent); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		//ตัดเงิน
		res, err := u.dockMoney(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("buy dock money", step.EventID, res, err)
			return nil, u.abortSaga(pctx, cfg, saga)
		}

		// เอาข้อมูลของ saga ไป save
		if err := u.finishStep(saga, res.TransactionId, ""); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
		}
	}

	//เอาของ เข้า กระเป๋า
	out := make([]*payment.PaymentTransferRes, 0, len(req.Items))
	for _, paid := range doneSteps(saga, stepDockMoney) {
		// สร้าง step เพื่อเอาใว้ track
		step := paymentStep{EventID: newEventID(), ItemID: paid.ItemID, Amount: paid.Amount, TransactionID: paid.TransactionID}
		if err := u.trackStep(saga, step, stepAddItem, stepSent); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
		}

		//ส่งข้อมูล Item เข้า kafka โดยมี topic inventory
		res, err := u.addItem(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("buy add item", step.EventID, res, err)
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		if err := u.finishStep(saga, paid.TransactionID, res.InventoryId); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
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

	saga.Status = sagaCompleted
	if err := u.saveSaga(saga); err != nil {
		return nil, u.abortSaga(pctx, cfg, saga)
	}
	return out, nil
}

func (u *paymentUsecase) SellItem(pctx context.Context, cfg *config.Config, playerId string, req *payment.ItemServiceReq) ([]*payment.PaymentTransferRes, error) {
	if err := u.prepareItems(pctx, cfg, req); err != nil {
		return nil, err
	}

	saga := &payment.Saga{ID: newEventID(), PlayerID: playerId, Action: "sell", Status: sagaRunning}
	if err := u.saveSaga(saga); err != nil {
		return nil, errors.New("error: sell item failed")
	}

	for _, item := range req.Items {
		step := paymentStep{EventID: newEventID(), ItemID: item.ItemId, Amount: item.Price}
		if err := u.trackStep(saga, step, stepRemoveItem, stepSent); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		res, err := u.removeItem(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("sell remove item", step.EventID, res, err)
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		if err := u.finishStep(saga, "", ""); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
		}
	}

	out := make([]*payment.PaymentTransferRes, 0, len(req.Items))
	for _, item := range doneSteps(saga, stepRemoveItem) {
		payout := sellPayout(item.Amount)
		if payout <= 0 {
			log.Printf("Error: sell payout is zero item_id=%s", item.ItemID)
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		step := paymentStep{EventID: newEventID(), ItemID: item.ItemID, Amount: payout}
		if err := u.trackStep(saga, step, stepAddMoney, stepSent); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		res, err := u.addMoney(pctx, cfg, playerId, step)
		if err != nil || res == nil || res.Error != "" {
			u.logStep("sell add money", step.EventID, res, err)
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		if err := u.finishStep(saga, res.TransactionId, ""); err != nil {
			return nil, u.abortSaga(pctx, cfg, saga)
		}
		out = append(out, &payment.PaymentTransferRes{
			EventId:       step.EventID,
			TransactionId: res.TransactionId,
			PlayerId:      playerId,
			ItemId:        item.ItemID,
			Amount:        item.Amount,
		})
	}

	saga.Status = sagaCompleted
	if err := u.saveSaga(saga); err != nil {
		return nil, u.abortSaga(pctx, cfg, saga)
	}
	return out, nil
}

// RecoverStaleSagas ชดเชย saga ที่ค้าง status running นานกว่า sagaStaleAfter (เรียกซ้ำจาก payment consumer loop)
func (u *paymentUsecase) RecoverStaleSagas(pctx context.Context, cfg *config.Config) {
	sagas, err := u.paymentRepository.ListStaleSagas(pctx, time.Now().Add(-sagaStaleAfter))
	if err != nil {
		log.Printf("Error: recover saga: %s", err.Error())
		return
	}
	for _, saga := range sagas {
		log.Printf("Info: recover stale saga %s action=%s", saga.ID, saga.Action)
		u.compensate(pctx, cfg, saga)
		saga.Status = sagaFailed
		if err := u.saveSaga(saga); err != nil {
			log.Printf("Error: mark recovered saga %s: %s", saga.ID, err.Error())
		}
	}
}

func (u *paymentUsecase) trackStep(saga *payment.Saga, step paymentStep, kind, status string) error {
	saga.Steps = append(saga.Steps, payment.SagaStep{
		EventID:       step.EventID,
		Kind:          kind,
		ItemID:        step.ItemID,
		Amount:        step.Amount,
		TransactionID: step.TransactionID,
		InventoryID:   step.InventoryID,
		Status:        status,
	})
	return u.saveSaga(saga)
}

func (u *paymentUsecase) finishStep(saga *payment.Saga, transactionID, inventoryID string) error {
	last := &saga.Steps[len(saga.Steps)-1]
	last.Status = stepDone
	if transactionID != "" {
		last.TransactionID = transactionID
	}
	if inventoryID != "" {
		last.InventoryID = inventoryID
	}
	return u.saveSaga(saga)
}

func (u *paymentUsecase) abortSaga(pctx context.Context, cfg *config.Config, saga *payment.Saga) error {
	u.compensate(pctx, cfg, saga)
	saga.Status = sagaFailed
	if err := u.saveSaga(saga); err != nil {
		log.Printf("Error: abort saga %s: %s", saga.ID, err.Error())
	}
	if saga.Action == "sell" {
		return errors.New("error: sell item failed")
	}
	return errors.New("error: buy item failed")
}

func (u *paymentUsecase) compensate(pctx context.Context, cfg *config.Config, saga *payment.Saga) {
	money := make([]paymentStep, 0)
	added := make([]paymentStep, 0)
	removed := make([]paymentStep, 0)
	for _, step := range saga.Steps {
		// rollback เฉพาะ step ที่ทำสำเร็จแล้ว (done) — ไม่ย้อน step ที่ค้างแค่ sent
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
		case stepDockMoney, stepAddMoney:
			money = append(money, recorded)
		case stepAddItem:
			added = append(added, recorded)
		case stepRemoveItem:
			removed = append(removed, recorded)
		}
	}
	u.rollbackAddedItems(pctx, cfg, added)
	u.rollbackRemovedItems(pctx, cfg, saga.PlayerID, removed)
	u.rollbackMoney(pctx, cfg, money)
}

func (u *paymentUsecase) saveSaga(saga *payment.Saga) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := u.paymentRepository.SaveSaga(ctx, saga); err != nil {
		log.Printf("Error: save saga %s: %s", saga.ID, err.Error())
		return err
	}
	return nil
}

func doneSteps(saga *payment.Saga, kind string) []paymentStep {
	steps := make([]paymentStep, 0)
	for _, step := range saga.Steps {
		if step.Kind != kind || step.Status != stepDone {
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
	if err := u.FindItemsInIds(pctx, cfg.Grpc.ItemUrl, req.Items); err != nil {
		return err
	}
	for _, item := range req.Items {
		if item.Price <= 0 {
			return errors.New("error: item price is invalid")
		}
	}
	return nil
}

func (u *paymentUsecase) dockMoney(pctx context.Context, cfg *config.Config, playerId string, step paymentStep) (*payment.PaymentTransferRes, error) {
	ch := u.register(step.EventID)
	defer u.unregister(step.EventID)

	if err := u.paymentRepository.DockedPlayerMoney(pctx, cfg, &player.CreatePlayerTransactionReq{
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

func (u *paymentUsecase) addItem(pctx context.Context, cfg *config.Config, playerId string, step paymentStep) (*payment.PaymentTransferRes, error) {
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

func (u *paymentUsecase) FindItemsInIds(pctx context.Context, grpcUrl string, req []*payment.ItemServiceReqDatum) error {
	setIds := make(map[string]bool)
	for _, v := range req {
		if !setIds[v.ItemId] {
			setIds[v.ItemId] = true
		}
	}

	itemData, err := u.paymentRepository.FindItemsInIds(pctx, grpcUrl, &itemPb.FindItemsInIdsReq{
		Ids: func() []string {
			itemIds := make([]string, 0)
			for k := range setIds {
				itemIds = append(itemIds, k)
			}
			return itemIds
		}(),
	})
	if err != nil {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
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
			log.Printf("Error: FindItemsInIds failed: item %s not found", req[i].ItemId)
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
