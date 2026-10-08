package inventoryRepository

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
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	queue "github.com/watcharaphong99/InwzaShop/pkg/kafka.go"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	InventoryRepositoryService interface {
		GetOffset(pctx context.Context) (int64, error)
		UpsertOffset(pctx context.Context, offset int64) error
		FindItemsInIds(pctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error)
		FindPlayerItems(pctx context.Context, filter primitive.D, opts []*options.FindOptions) ([]*inventory.Inventory, error)
		CountPlayerItems(pctx context.Context, playerId string) (int64, error)
		AddPlayerItemRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error
		RemovePlayerItemRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error
		InsertOnePlayerItem(pctx context.Context, req *inventory.Inventory) (primitive.ObjectID, error)
		DeleteOneInventory(pctx context.Context, inventoryId string) error
		FindOnePlayerItem(pctx context.Context, playerId, itemId string) bool
		DeleteOnePlayerItem(pctx context.Context, playerId, itemId string) error
		RemovePlayerItemByEvent(pctx context.Context, playerId, itemId, eventID string) error
		RollbackAddedItem(pctx context.Context, inventoryID, eventID string) error
		RollbackRemovedItem(pctx context.Context, eventID string) error
	}

	inventoryRepository struct {
		db *mongo.Client
	}
)

func NewInventoryRepository(db *mongo.Client) InventoryRepositoryService {
	repo := &inventoryRepository{db}
	repo.ensureIndexes()
	return repo
}

func (r *inventoryRepository) inventoryDbConn(pctx context.Context) *mongo.Database {
	return r.db.Database("inventory_db")
}

func (r *inventoryRepository) GetOffset(pctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory_queue")

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

func (r *inventoryRepository) UpsertOffset(pctx context.Context, offset int64) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory_queue")

	result, err := col.UpdateOne(ctx, bson.M{}, bson.M{"$set": bson.M{"offset": offset}}, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Error: UpserOffset failed: %s", err.Error())
		return errors.New("error: UpserOffset failed")
	}
	log.Printf("Info: UpserOffset result: %v", result)

	return nil
}

func (r *inventoryRepository) FindItemsInIds(pctx context.Context, grpcUrl string, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
	ctx, cancel := context.WithTimeout(pctx, 30*time.Second)
	defer cancel()

	jwtauth.SetApiKeyInContext(&ctx)
	conn, err := grpccon.NewGrpcClient(grpcUrl)
	if err != nil {
		log.Printf("Error: gRPC connection failed: %s", err.Error())
		return nil, errors.New("error: gRPC connection failed")
	}

	result, err := conn.Item().FindItemsInIds(ctx, req)
	if err != nil {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
		return nil, errors.New("error: items not found")
	}

	if result == nil {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
		return nil, errors.New("error: items not found")
	}

	if result.Items == nil {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
		return nil, errors.New("error: items not found")
	}

	if len(result.Items) == 0 {
		log.Printf("Error: FindItemsInIds failed: %s", err.Error())
		return nil, errors.New("error: items not found")
	}

	return result, nil
}

func (r *inventoryRepository) FindPlayerItems(pctx context.Context, filter primitive.D, opts []*options.FindOptions) ([]*inventory.Inventory, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory")

	cursors, err := col.Find(ctx, filter, opts...)
	if err != nil {
		log.Printf("Error: FindPlayerItems failed: %s", err.Error())
		return nil, errors.New("error: player items not found")
	}

	results := make([]*inventory.Inventory, 0)
	for cursors.Next(ctx) {
		result := new(inventory.Inventory)
		if err := cursors.Decode(result); err != nil {
			log.Printf("Error: FindPlayerItems failed: %s", err.Error())
			return nil, errors.New("error: player items not found")
		}

		results = append(results, result)
	}

	return results, nil
}

func (r *inventoryRepository) CountPlayerItems(pctx context.Context, playerId string) (int64, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory")

	count, err := col.CountDocuments(ctx, bson.M{
		"player_id":        playerId,
		"removed_event_id": bson.M{"$exists": false},
	})
	if err != nil {
		log.Printf("Error: CountPlayerItems failed: %s", err.Error())
		return -1, errors.New("error: count player items failed")
	}

	return count, nil
}

func (r *inventoryRepository) InsertOnePlayerItem(pctx context.Context, req *inventory.Inventory) (primitive.ObjectID, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory")

	if req.EventId != "" && r.isEventCancelled(ctx, req.EventId) {
		return primitive.NilObjectID, inventory.ErrEventCancelled
	}

	result, err := col.InsertOne(ctx, req)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) && req.EventId != "" {
			return r.existingPlayerItem(ctx, req)
		}
		log.Printf("Error: InsertOnePlayerItem failed: %s", err.Error())
		return primitive.NilObjectID, errors.New("error: insert player item failed")
	}

	insertedID := result.InsertedID.(primitive.ObjectID)
	if req.EventId != "" && r.isEventCancelled(ctx, req.EventId) {
		if _, delErr := col.DeleteOne(ctx, bson.M{"event_id": req.EventId}); delErr != nil {
			log.Printf("Error: delete cancelled item: %s", delErr.Error())
		}
		return primitive.NilObjectID, inventory.ErrEventCancelled
	}

	return insertedID, nil
}

func (r *inventoryRepository) DeleteOneInventory(pctx context.Context, inventoryId string) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory")

	result, err := col.DeleteOne(ctx, bson.M{"_id": utils.ConvertToObjectId(inventoryId)})
	if err != nil {
		log.Printf("Error: DeleteOneInventory failed: %s", err.Error())
		return errors.New("error: delete one inventory failed")
	}
	log.Printf("DeleteOneInventory result: %v", result)

	return nil
}

func (r *inventoryRepository) AddPlayerItemRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: AddPlayerItemRes failed: %s", err.Error())
		return errors.New("error: docked player money res failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"payment",
		"buy",
		reqInBytes,
	); err != nil {
		log.Printf("Error: AddPlayerItemRes failed: %s", err.Error())
		return errors.New("error: docked player money res failed")
	}

	return nil
}

func (r *inventoryRepository) FindOnePlayerItem(pctx context.Context, playerId, itemId string) bool {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory")

	result := new(inventory.Inventory)

	if err := col.FindOne(ctx, bson.M{
		"player_id":        playerId,
		"item_id":          itemId,
		"removed_event_id": bson.M{"$exists": false},
	}).Decode(result); err != nil {
		log.Printf("Error: FindOnePlayerItem failed: %s", err.Error())
		return false
	}
	return true
}

func (r *inventoryRepository) DeleteOnePlayerItem(pctx context.Context, playerId, itemId string) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	col := db.Collection("players_inventory")

	result, err := col.DeleteOne(ctx, bson.M{"player_id": playerId, "item_id": itemId})
	if err != nil {
		log.Printf("Error: DeleteOnePlayerItem failed: %s", err.Error())
		return errors.New("error: delete one player item failed")
	}
	log.Printf("DeleteOnePlayerItem result: %v", result)

	return nil
}

func (r *inventoryRepository) RemovePlayerItemRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: RemovePlayerItemRes failed: %s", err.Error())
		return errors.New("error: docked player money res failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"payment",
		"sell",
		reqInBytes,
	); err != nil {
		log.Printf("Error: RemovePlayerItemRes failed: %s", err.Error())
		return errors.New("error: docked player money res failed")
	}

	return nil
}

func (r *inventoryRepository) RemovePlayerItemByEvent(pctx context.Context, playerId, itemId, eventID string) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	if eventID == "" {
		return errors.New("error: event_id is required")
	}
	if r.isEventCancelled(ctx, eventID) {
		_ = r.clearRemoval(ctx, eventID)
		return inventory.ErrEventCancelled
	}
	if r.removalExists(ctx, eventID) {
		return nil
	}

	col := r.inventoryDbConn(ctx).Collection("players_inventory")
	err := col.FindOneAndUpdate(
		ctx,
		bson.M{
			"player_id":        playerId,
			"item_id":          itemId,
			"removed_event_id": bson.M{"$exists": false},
		},
		bson.M{"$set": bson.M{"removed_event_id": eventID}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		if r.removalExists(ctx, eventID) {
			return nil
		}
		return inventory.ErrItemNotFound
	}
	if mongo.IsDuplicateKeyError(err) {
		if r.removalExists(ctx, eventID) {
			return nil
		}
		log.Printf("Error: RemovePlayerItemByEvent duplicate: %s", err.Error())
		return errors.New("error: remove player item failed")
	}
	if err != nil {
		log.Printf("Error: RemovePlayerItemByEvent failed: %s", err.Error())
		return errors.New("error: remove player item failed")
	}
	if r.isEventCancelled(ctx, eventID) {
		_ = r.clearRemoval(ctx, eventID)
		return inventory.ErrEventCancelled
	}
	return nil
}

func (r *inventoryRepository) RollbackAddedItem(pctx context.Context, inventoryID, eventID string) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	col := r.inventoryDbConn(ctx).Collection("players_inventory")
	if eventID != "" {
		if err := r.markEventCancelled(ctx, eventID); err != nil {
			return err
		}
		if _, err := col.DeleteOne(ctx, bson.M{"event_id": eventID}); err != nil {
			log.Printf("Error: delete added item by event_id: %s", err.Error())
			return errors.New("error: rollback add player item failed")
		}
	}
	if inventoryID == "" {
		return nil
	}
	if _, err := utils.ParseObjectId(inventoryID); err != nil {
		return nil
	}
	return r.DeleteOneInventory(pctx, inventoryID)
}

func (r *inventoryRepository) RollbackRemovedItem(pctx context.Context, eventID string) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	if eventID == "" {
		return errors.New("error: event_id is required")
	}
	if err := r.markEventCancelled(ctx, eventID); err != nil {
		return err
	}
	return r.clearRemoval(ctx, eventID)
}

func (r *inventoryRepository) existingPlayerItem(ctx context.Context, req *inventory.Inventory) (primitive.ObjectID, error) {
	existing := new(inventory.Inventory)
	err := r.inventoryDbConn(ctx).Collection("players_inventory").FindOne(ctx, bson.M{"event_id": req.EventId}).Decode(existing)
	if err != nil {
		log.Printf("Error: find item by event_id: %s", err.Error())
		return primitive.NilObjectID, errors.New("error: insert player item failed")
	}
	if existing.PlayerId != req.PlayerId || existing.ItemId != req.ItemId || existing.RemovedEventId != "" {
		return primitive.NilObjectID, errors.New("error: event_id already used")
	}
	if r.isEventCancelled(ctx, req.EventId) {
		if _, delErr := r.inventoryDbConn(ctx).Collection("players_inventory").DeleteOne(ctx, bson.M{"event_id": req.EventId}); delErr != nil {
			log.Printf("Error: delete cancelled item: %s", delErr.Error())
		}
		return primitive.NilObjectID, inventory.ErrEventCancelled
	}
	return existing.Id, nil
}

func (r *inventoryRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db := r.inventoryDbConn(ctx)
	items := db.Collection("players_inventory")
	if _, err := items.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "event_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	}); err != nil {
		log.Printf("Error: inventory event index: %s", err.Error())
	}
	if _, err := items.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "removed_event_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	}); err != nil {
		log.Printf("Error: inventory removal index: %s", err.Error())
	}
	if _, err := db.Collection("inventory_event_cancels").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "event_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		log.Printf("Error: inventory cancel index: %s", err.Error())
	}
}

func (r *inventoryRepository) markEventCancelled(ctx context.Context, eventID string) error {
	_, err := r.inventoryDbConn(ctx).Collection("inventory_event_cancels").UpdateOne(
		ctx,
		bson.M{"event_id": eventID},
		bson.M{"$setOnInsert": bson.M{"event_id": eventID}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Error: mark inventory event cancelled: %s", err.Error())
		return errors.New("error: cancel inventory event failed")
	}
	return nil
}

func (r *inventoryRepository) isEventCancelled(ctx context.Context, eventID string) bool {
	err := r.inventoryDbConn(ctx).Collection("inventory_event_cancels").FindOne(ctx, bson.M{"event_id": eventID}).Err()
	if err == nil {
		return true
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false
	}
	log.Printf("Error: read inventory cancel: %s", err.Error())
	return true
}

func (r *inventoryRepository) removalExists(ctx context.Context, eventID string) bool {
	err := r.inventoryDbConn(ctx).Collection("players_inventory").FindOne(ctx, bson.M{"removed_event_id": eventID}).Err()
	if err == nil {
		return true
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false
	}
	log.Printf("Error: read inventory removal: %s", err.Error())
	return false
}

func (r *inventoryRepository) clearRemoval(ctx context.Context, eventID string) error {
	_, err := r.inventoryDbConn(ctx).Collection("players_inventory").UpdateOne(
		ctx,
		bson.M{"removed_event_id": eventID},
		bson.M{"$unset": bson.M{"removed_event_id": ""}},
	)
	if err != nil {
		log.Printf("Error: clear inventory removal: %s", err.Error())
		return errors.New("error: rollback remove player item failed")
	}
	return nil
}

func grpcItemError(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return "error: find items in ids failed"
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return "error: item service is unavailable"
	case codes.Unauthenticated, codes.PermissionDenied:
		return "error: grpc authorization failed"
	default:
		return "error: find items in ids failed"
	}
}
