package paymentRepository

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/inventory"
	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"
	"github.com/watcharaphong99/InwzaShop/modules/models"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
	"github.com/watcharaphong99/InwzaShop/modules/player"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	queue "github.com/watcharaphong99/InwzaShop/pkg/kafka.go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	PaymentRepositoryService interface {
		GetOffset(pctx context.Context) (int64, error)
		UpsertOffset(pctx context.Context, offset int64) error
		FindProductsInCatalog(pctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error)
		CapturePayment(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq) error
		RollbackTransaction(pctx context.Context, cfg *config.Config, req *player.RollbackPlayerTransactionReq) error
		AddPlayerItem(pctx context.Context, cfg *config.Config, req *inventory.UpdateInventoryReq) error
		RollbackAddPlayerItem(pctx context.Context, cfg *config.Config, req *inventory.RollbackPlayerInventoryReq) error
		RemovePlayerItem(pctx context.Context, cfg *config.Config, req *inventory.UpdateInventoryReq) error
		RollbackRemovePlayerItem(pctx context.Context, cfg *config.Config, req *inventory.RollbackPlayerInventoryReq) error
		AddPlayerMoney(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq) error
		SaveOrderWorkflow(pctx context.Context, workflow *payment.OrderWorkflow) error
		ListStaleOrderWorkflows(pctx context.Context, olderThan time.Time) ([]*payment.OrderWorkflow, error)
	}

	paymentRepository struct {
		db *mongo.Client
	}
)

func NewPaymentRepository(db *mongo.Client) PaymentRepositoryService {
	return &paymentRepository{db}
}

func (r *paymentRepository) paymentDbConn() *mongo.Database {
	return r.db.Database("payment_db")
}

func (r *paymentRepository) GetOffset(pctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.paymentDbConn()
	col := db.Collection("payment_queue")

	result := new(models.KafkaOffset)
	if err := col.FindOne(ctx, bson.M{}).Decode(result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return -1, nil
		}
		log.Printf("Error: GetOffset failed: %s", err.Error())
		return -1, errors.New("error: GetOffset failed")
	}

	return result.Offset, nil
}

func (r *paymentRepository) UpsertOffset(pctx context.Context, offset int64) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.paymentDbConn()
	col := db.Collection("payment_queue")

	result, err := col.UpdateOne(ctx, bson.M{}, bson.M{"$set": bson.M{"offset": offset}}, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Error: UpserOffset failed: %s", err.Error())
		return errors.New("error: UpserOffset failed")
	}
	log.Printf("Info: UpserOffset result: %v", result)

	return nil
}

// FindProductsInCatalog เรียก ProductCatalog / PricingService (Item gRPC FindItemsInIds)
func (r *paymentRepository) FindProductsInCatalog(pctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
	ctx, cancel := context.WithTimeout(pctx, 30*time.Second)
	defer cancel()

	jwtauth.SetApiKeyInContext(&ctx)
	conn, err := grpccon.NewGrpcClient(grpcUrl)
	if err != nil {
		log.Printf("Error: gRPC connection failed: %s", err.Error())
		return nil, errors.New("error: gRPC connection failed")
	}
	defer conn.Close()

	result, err := conn.Item().FindItemsInIds(ctx, req)
	if err != nil {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
		return nil, errors.New("error: items not found")
	}

	if result == nil || len(result.Items) == 0 {
		log.Printf("Error: FindItemsInIds failed: empty item result")
		return nil, errors.New("error: items not found")
	}

	return result, nil
}

// CapturePayment ส่งคำสั่งหักเงิน (เดิม DockedPlayerMoney / dock_money)
func (r *paymentRepository) CapturePayment(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: CapturePayment failed: %s", err.Error())
		return errors.New("error: capture payment failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"player",
		"buy",
		reqInBytes,
	); err != nil {
		log.Printf("Error: CapturePayment failed: %s", err.Error())
		return errors.New("error: capture payment failed")
	}

	return nil
}
func (r *paymentRepository) AddPlayerMoney(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: AddPlayerMoney failed: %s", err.Error())
		return errors.New("error: add player money failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"player",
		"sell",
		reqInBytes,
	); err != nil {
		log.Printf("Error: AddPlayerMoney failed: %s", err.Error())
		return errors.New("error: add player money failed")
	}

	return nil
}

func (r *paymentRepository) RollbackTransaction(pctx context.Context, cfg *config.Config, req *player.RollbackPlayerTransactionReq) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: DockedPlayerMoney failed: %s", err.Error())
		return errors.New("error: rollback player transaction failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"player",
		"rtransaction",
		reqInBytes,
	); err != nil {
		log.Printf("Error: DockedPlayerMoney failed: %s", err.Error())
		return errors.New("error: rollback player transaction failed")
	}

	return nil
}

func (r *paymentRepository) AddPlayerItem(pctx context.Context, cfg *config.Config, req *inventory.UpdateInventoryReq) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: AddPlayerItem failed: %s", err.Error())
		return errors.New("error: add player item failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"inventory",
		"buy",
		reqInBytes,
	); err != nil {
		log.Printf("Error: AddPlayerItem failed: %s", err.Error())
		return errors.New("error: add player item failed")
	}

	return nil
}

func (r *paymentRepository) RollbackAddPlayerItem(pctx context.Context, cfg *config.Config, req *inventory.RollbackPlayerInventoryReq) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: RollbackAddPlayerItem failed: %s", err.Error())
		return errors.New("error: rollback add player item failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"inventory",
		"radd",
		reqInBytes,
	); err != nil {
		log.Printf("Error: RollbackAddPlayerItem failed: %s", err.Error())
		return errors.New("error: rollback add player item failed")
	}

	return nil
}

func (r *paymentRepository) RemovePlayerItem(pctx context.Context, cfg *config.Config, req *inventory.UpdateInventoryReq) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: RemovePlayerItem failed: %s", err.Error())
		return errors.New("error: remove player item failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"inventory",
		"sell",
		reqInBytes,
	); err != nil {
		log.Printf("Error: RemovePlayerItem failed: %s", err.Error())
		return errors.New("error: remove player item failed")
	}

	return nil
}

func (r *paymentRepository) RollbackRemovePlayerItem(pctx context.Context, cfg *config.Config, req *inventory.RollbackPlayerInventoryReq) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: RollbackRemovePlayerItem failed: %s", err.Error())
		return errors.New("error: rollback remove player item failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"inventory",
		"rremove",
		reqInBytes,
	); err != nil {
		log.Printf("Error: RollbackRemovePlayerItem failed: %s", err.Error())
		return errors.New("error: rollback remove player item failed")
	}

	return nil
}

func (r *paymentRepository) SaveOrderWorkflow(pctx context.Context, workflow *payment.OrderWorkflow) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	workflow.UpdatedAt = time.Now()
	_, err := r.paymentDbConn().Collection("order_workflows").ReplaceOne(
		ctx,
		bson.M{"_id": workflow.ID},
		workflow,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Error: SaveOrderWorkflow failed: %s", err.Error())
		return errors.New("error: save order workflow failed")
	}
	return nil
}

func (r *paymentRepository) ListStaleOrderWorkflows(pctx context.Context, olderThan time.Time) ([]*payment.OrderWorkflow, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	filter := bson.M{
		"status":     "running",
		"updated_at": bson.M{"$lt": olderThan},
	}

	workflows := make([]*payment.OrderWorkflow, 0)
	for _, collection := range []string{"order_workflows", "payment_sagas"} {
		cursor, err := r.paymentDbConn().Collection(collection).Find(ctx, filter)
		if err != nil {
			log.Printf("Error: ListStaleOrderWorkflows %s: %s", collection, err.Error())
			return nil, errors.New("error: list order workflow failed")
		}
		for cursor.Next(ctx) {
			wf := new(payment.OrderWorkflow)
			if err := cursor.Decode(wf); err != nil {
				cursor.Close(ctx)
				log.Printf("Error: decode order workflow: %s", err.Error())
				return nil, errors.New("error: list order workflow failed")
			}
			workflows = append(workflows, wf)
		}
		if err := cursor.Err(); err != nil {
			cursor.Close(ctx)
			log.Printf("Error: ListStaleOrderWorkflows cursor: %s", err.Error())
			return nil, errors.New("error: list order workflow failed")
		}
		cursor.Close(ctx)
	}
	return workflows, nil
}
