package playerRepository

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"strings"
	"time"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/models"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
	"github.com/watcharaphong99/InwzaShop/modules/player"
	queue "github.com/watcharaphong99/InwzaShop/pkg/kafka.go"
	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	PlayerRepositoryService interface {
		GetOffset(pctx context.Context) (int64, error)
		UpsertOffset(pctx context.Context, offset int64) error
		IsUniquePlayer(pctx context.Context, email, username string) bool
		InsertOnePlayer(pctx context.Context, req *player.Player) (primitive.ObjectID, error)
		FindOnePlayerProfine(pctx context.Context, playerId string) (*player.PlayerProfileBson, error)
		InsertOnePlayerTranscation(pctx context.Context, req *player.PlayerTransaction) (primitive.ObjectID, error)
		GetPlayerSavingAccount(pctx context.Context, playerId string) (*player.PlayerSavingAccount, error)
		FindOnePlayerCredential(pctx context.Context, email string) (*player.Player, error)
		FindOnePlayerProfileTokenRefresh(pctx context.Context, player_id string) (*player.Player, error)
		DeleteOnePlayerTransaction(pctx context.Context, transactionId string) error
		CancelPlayerTransaction(pctx context.Context, eventID, transactionID string) error
		DockedPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error
		AddPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error
	}

	playerRepository struct {
		db    *mongo.Client
		cache *rediscon.Client
	}
)

func NewPlayerRepository(db *mongo.Client, cache *rediscon.Client) PlayerRepositoryService {
	repo := &playerRepository{db: db, cache: cache}
	repo.ensureIndexes()
	return repo
}

func (r *playerRepository) playerDbConn() *mongo.Database {
	return r.db.Database("player_db")
}

func (r *playerRepository) GetOffset(pctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.playerDbConn()
	col := db.Collection("player_transactions_queue")

	result := new(models.KafkaOffset)
	if err := col.FindOne(ctx, bson.M{"_id": models.KafkaOffsetID}).Decode(result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return -1, nil
		}
		log.Printf("Error: GetOffset failed: %s", err.Error())
		return -1, errors.New("error: GetOffset failed")
	}

	return result.Offset, nil
}

func (r *playerRepository) UpsertOffset(pctx context.Context, offset int64) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.playerDbConn()
	col := db.Collection("player_transactions_queue")

	result, err := col.UpdateOne(ctx, bson.M{"_id": models.KafkaOffsetID}, bson.M{"$max": bson.M{"offset": offset}}, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Error: UpsertOffset failed: %s", err.Error())
		return errors.New("error: UpsertOffset failed")
	}
	log.Printf("Info: UpsertOffset result: %v", result)

	return nil
}

func (r *playerRepository) IsUniquePlayer(pctx context.Context, email, username string) bool {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn()
	col := db.Collection("players")

	player := new(player.Player)
	if err := col.FindOne(
		ctx,
		bson.M{"$or": []bson.M{
			{"username": username},
			{"email": email},
		}},
	).Decode(player); err != nil {
		log.Printf("Error: IsUniquePlayer: %s", err.Error())
		return true
	}
	return false
}

func (r *playerRepository) InsertOnePlayer(pctx context.Context, req *player.Player) (primitive.ObjectID, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	db := r.playerDbConn()
	col := db.Collection("players")

	playerId, err := col.InsertOne(ctx, req)
	if err != nil {
		log.Printf("Error: InsertOnePlayer: %s", err.Error())
		if mongo.IsDuplicateKeyError(err) {
			return primitive.NewObjectID(), errors.New("error: email or username already exist")
		}
		return primitive.NewObjectID(), errors.New("error: insert one player failed")
	}

	insertedID := playerId.InsertedID.(primitive.ObjectID)
	r.cache.Del(ctx, rediscon.PlayerProfileKey(insertedID.Hex()))
	return insertedID, nil

}

func (r *playerRepository) FindOnePlayerProfine(pctx context.Context, playerId string) (*player.PlayerProfileBson, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn()
	col := db.Collection("players")

	result := new(player.PlayerProfileBson)
	playerKey := strings.TrimPrefix(playerId, "player:")
	lookupID := utils.ConvertToObjectId(playerKey)
	if objectId, err := utils.ParseObjectId(playerKey); err == nil {
		lookupID = objectId
		if r.cache.GetJSON(ctx, rediscon.PlayerProfileKey(objectId.Hex()), result) {
			return result, nil
		}
	}

	log.Println("playerId", playerId)

	if err := col.FindOne(
		ctx,
		bson.M{"_id": lookupID},
		options.FindOne().SetProjection(
			bson.M{
				"_id":        1,
				"email":      1,
				"username":   1,
				"created_at": 1,
				"updated_at": 1,
			},
		),
	).Decode(result); err != nil {
		log.Printf("Error: FindOnePlayerProfile: %s", err.Error())
		return nil, errors.New("error: player profile not found")
	}

	r.cache.SetJSON(ctx, rediscon.PlayerProfileKey(result.Id.Hex()), result, rediscon.PlayerProfileTTL)

	return result, nil

}

func (r *playerRepository) InsertOnePlayerTranscation(pctx context.Context, req *player.PlayerTransaction) (primitive.ObjectID, error) {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	if req.EventId != "" && r.isTransactionCancelled(ctx, req.EventId) {
		return primitive.NilObjectID, player.ErrEventCancelled
	}

	// หัก/เติมเงิน: อัปเดต player_wallets แบบ atomic คู่กับ insert ledger ใน Mongo transaction
	session, err := r.db.StartSession()
	if err != nil {
		log.Printf("Error: player transaction session: %s", err.Error())
		return primitive.NilObjectID, errors.New("error: insert one player transaction failed")
	}
	defer session.EndSession(ctx)

	var insertedID primitive.ObjectID
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		txCol := r.playerDbConn().Collection("player_transactions")
		walletCol := r.playerDbConn().Collection("player_wallets")

		if req.EventId != "" && r.isTransactionCancelled(sc, req.EventId) {
			return nil, player.ErrEventCancelled
		}

		// idempotent retry: มี ledger ของ event นี้แล้ว — ไม่แตะ wallet ซ้ำ
		if req.EventId != "" {
			existing := new(player.PlayerTransaction)
			if err := txCol.FindOne(sc, bson.M{"event_id": req.EventId}).Decode(existing); err == nil {
				if !sameEventTransaction(existing, req) {
					return nil, errors.New("error: event_id already used")
				}
				if r.isTransactionCancelled(sc, req.EventId) {
					return nil, player.ErrEventCancelled
				}
				insertedID = existing.Id
				return nil, nil
			} else if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("error: insert one player transaction failed")
			}
		}

		if err := r.applyWalletDelta(sc, walletCol, req.PlayerId, req.Amount); err != nil {
			return nil, err
		}

		result, err := txCol.InsertOne(sc, req)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) && req.EventId != "" {
				// concurrent duplicate event_id: ย้อน wallet ที่เพิ่งหัก/เติม แล้วคืน transaction เดิม
				if revErr := r.applyWalletDelta(sc, walletCol, req.PlayerId, -req.Amount); revErr != nil {
					log.Printf("Error: reverse wallet on duplicate event: %s", revErr.Error())
				}
				id, dupErr := r.sameEventTransactionOrConflict(sc, txCol, req)
				if dupErr != nil {
					return nil, dupErr
				}
				insertedID = id
				return nil, nil
			}
			log.Printf("Error: InseartOnePlayerTransaction: %s", err.Error())
			return nil, errors.New("error: insert one player transaction failed")
		}

		insertedID = result.InsertedID.(primitive.ObjectID)
		if req.EventId != "" && r.isTransactionCancelled(sc, req.EventId) {
			if delErr := r.deleteTransactionByEventID(sc, req.EventId); delErr != nil {
				log.Printf("Error: delete cancelled transaction: %s", delErr.Error())
			}
			if revErr := r.applyWalletDelta(sc, walletCol, req.PlayerId, -req.Amount); revErr != nil {
				log.Printf("Error: reverse wallet on cancelled event: %s", revErr.Error())
			}
			return nil, player.ErrEventCancelled
		}
		return nil, nil
	})
	if err != nil {
		if errors.Is(err, player.ErrEventCancelled) || errors.Is(err, player.ErrNotEnoughMoney) {
			return primitive.NilObjectID, err
		}
		return primitive.NilObjectID, err
	}

	if insertedID == primitive.NilObjectID {
		return primitive.NilObjectID, errors.New("error: insert one player transaction failed")
	}

	log.Printf("Result: InseartOnePlayerTransaction: %v", insertedID)
	return insertedID, nil
}

func (r *playerRepository) sameEventTransactionOrConflict(ctx context.Context, col *mongo.Collection, req *player.PlayerTransaction) (primitive.ObjectID, error) {
	if req.EventId == "" {
		return primitive.NilObjectID, errors.New("error: insert one player transaction failed")
	}

	existing := new(player.PlayerTransaction)
	if err := col.FindOne(ctx, bson.M{"event_id": req.EventId}).Decode(existing); err != nil {
		log.Printf("Error: find player transaction by event_id: %s", err.Error())
		return primitive.NilObjectID, errors.New("error: insert one player transaction failed")
	}
	if !sameEventTransaction(existing, req) {
		log.Printf("Error: event_id already used: %s", req.EventId)
		return primitive.NilObjectID, errors.New("error: event_id already used")
	}
	if r.isTransactionCancelled(ctx, req.EventId) {
		if err := r.deleteTransactionByEventID(ctx, req.EventId); err != nil {
			log.Printf("Error: delete cancelled transaction: %s", err.Error())
		}
		return primitive.NilObjectID, player.ErrEventCancelled
	}

	log.Printf("Info: InseartOnePlayerTransaction duplicate event_id: %s", req.EventId)
	return existing.Id, nil
}

func (r *playerRepository) CancelPlayerTransaction(pctx context.Context, eventID, transactionID string) error {
	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	if eventID != "" {
		if err := r.markTransactionCancelled(ctx, eventID); err != nil {
			return err
		}
		if err := r.deleteTransactionByEventID(ctx, eventID); err != nil {
			return err
		}
	} else if transactionID != "" {
		if err := r.DeleteOnePlayerTransaction(pctx, transactionID); err != nil {
			log.Printf("Error: delete transaction %s: %s", transactionID, err.Error())
		}
	}
	return nil
}

func (r *playerRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db := r.playerDbConn()
	if _, err := db.Collection("player_transactions").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "event_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	}); err != nil {
		log.Printf("Error: player transaction event index: %s", err.Error())
	}
	if _, err := db.Collection("player_tx_cancels").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "event_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		log.Printf("Error: player cancel index: %s", err.Error())
	}
	// wallet ต่อ player — ใช้ FindOneAndUpdate หักเงินแบบ atomic
	if _, err := db.Collection("player_wallets").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "player_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		log.Printf("Error: player wallet index: %s", err.Error())
	}
}

func (r *playerRepository) markTransactionCancelled(ctx context.Context, eventID string) error {
	_, err := r.playerDbConn().Collection("player_tx_cancels").UpdateOne(
		ctx,
		bson.M{"event_id": eventID},
		bson.M{"$setOnInsert": bson.M{"event_id": eventID}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Error: mark transaction cancelled: %s", err.Error())
		return errors.New("error: cancel transaction failed")
	}
	return nil
}

func (r *playerRepository) isTransactionCancelled(ctx context.Context, eventID string) bool {
	err := r.playerDbConn().Collection("player_tx_cancels").FindOne(ctx, bson.M{"event_id": eventID}).Err()
	if err == nil {
		return true
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false
	}
	log.Printf("Error: read transaction cancel: %s", err.Error())
	return true
}

func (r *playerRepository) deleteTransactionByEventID(ctx context.Context, eventID string) error {
	txCol := r.playerDbConn().Collection("player_transactions")
	walletCol := r.playerDbConn().Collection("player_wallets")

	tx := new(player.PlayerTransaction)
	if err := txCol.FindOne(ctx, bson.M{"event_id": eventID}).Decode(tx); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil
		}
		log.Printf("Error: find transaction by event_id: %s", err.Error())
		return errors.New("error: delete transaction failed")
	}

	if _, err := txCol.DeleteOne(ctx, bson.M{"event_id": eventID}); err != nil {
		log.Printf("Error: delete transaction by event_id: %s", err.Error())
		return errors.New("error: delete transaction failed")
	}

	// rollback ledger แล้ว sync wallet ให้ตรง (ลบ amount -100 → คืน +100)
	if err := r.applyWalletDelta(ctx, walletCol, tx.PlayerId, -tx.Amount); err != nil {
		log.Printf("Error: wallet sync on delete event_id=%s: %s", eventID, err.Error())
	}
	return nil
}

// applyWalletDelta อัปเดตยอดใน player_wallets; หักเงินใช้เงื่อนไข balance >= จำนวนหัก
func (r *playerRepository) applyWalletDelta(ctx context.Context, walletCol *mongo.Collection, playerId string, delta float64) error {
	if delta == 0 {
		return nil
	}
	if err := r.ensurePlayerWallet(ctx, walletCol, playerId); err != nil {
		return err
	}

	if delta < 0 {
		deduct := math.Abs(delta)
		res := walletCol.FindOneAndUpdate(
			ctx,
			bson.M{"player_id": playerId, "balance": bson.M{"$gte": deduct}},
			bson.M{"$inc": bson.M{"balance": delta}},
		)
		if err := res.Err(); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return player.ErrNotEnoughMoney
			}
			log.Printf("Error: wallet deduct: %s", err.Error())
			return errors.New("error: insert one player transaction failed")
		}
		return nil
	}

	_, err := walletCol.UpdateOne(ctx, bson.M{"player_id": playerId}, bson.M{"$inc": bson.M{"balance": delta}})
	if err != nil {
		log.Printf("Error: wallet credit: %s", err.Error())
		return errors.New("error: insert one player transaction failed")
	}
	return nil
}

func (r *playerRepository) ensurePlayerWallet(ctx context.Context, walletCol *mongo.Collection, playerId string) error {
	if err := walletCol.FindOne(ctx, bson.M{"player_id": playerId}).Err(); err == nil {
		return nil
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("Error: ensure wallet find: %s", err.Error())
		return errors.New("error: insert one player transaction failed")
	}

	balance, err := r.ledgerBalance(ctx, playerId)
	if err != nil {
		return err
	}

	_, err = walletCol.UpdateOne(
		ctx,
		bson.M{"player_id": playerId},
		bson.M{"$setOnInsert": bson.M{"player_id": playerId, "balance": balance}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Error: ensure wallet upsert: %s", err.Error())
		return errors.New("error: insert one player transaction failed")
	}
	return nil
}

func (r *playerRepository) ledgerBalance(ctx context.Context, playerId string) (float64, error) {
	col := r.playerDbConn().Collection("player_transactions")
	pipeline := bson.A{
		bson.D{{Key: "$match", Value: bson.D{{Key: "player_id", Value: playerId}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "balance", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
		}}},
	}

	cursor, err := col.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Error: ledger balance aggregate: %s", err.Error())
		return 0, errors.New("error: insert one player transaction failed")
	}
	defer cursor.Close(ctx)

	var row struct {
		Balance float64 `bson:"balance"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&row); err != nil {
			return 0, errors.New("error: insert one player transaction failed")
		}
		return row.Balance, nil
	}
	return 0, nil
}

func sameEventTransaction(existing, incoming *player.PlayerTransaction) bool {
	return existing.PlayerId == incoming.PlayerId && existing.Amount == incoming.Amount
}

func (r *playerRepository) GetPlayerSavingAccount(pctx context.Context, playerId string) (*player.PlayerSavingAccount, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn()
	walletCol := db.Collection("player_wallets")

	log.Printf("playerId at Repository: %s", playerId)

	// อ่านจาก wallet ก่อน ( sync กับหักเงิน atomic ); ไม่มี wallet ค่อย aggregate ledger เก่า
	wallet := new(player.PlayerWallet)
	if err := walletCol.FindOne(ctx, bson.M{"player_id": playerId}).Decode(wallet); err == nil {
		return &player.PlayerSavingAccount{PlayerId: playerId, Balance: wallet.Balance}, nil
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("Error: GetPlayerSavingAccount wallet: %s", err.Error())
		return nil, errors.New("error: failed to get player saving account")
	}

	col := db.Collection("player_transactions")

	filter := bson.A{
		bson.D{{Key: "$match", Value: bson.D{{Key: "player_id", Value: playerId}}}},
		bson.D{
			{Key: "$group",
				Value: bson.D{
					{Key: "_id", Value: "$player_id"},
					{Key: "balance", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
				},
			},
		},
		bson.D{
			{Key: "$project",
				Value: bson.D{
					{Key: "player_id", Value: "$_id"},
					{Key: "_id", Value: 0},
					{Key: "balance", Value: 1},
				},
			},
		},
	}

	cursor, err := col.Aggregate(ctx, filter)

	log.Printf("print cursor: %v", cursor)
	if err != nil {
		log.Printf("Error: GetPlayerSavingAccount: %s", err.Error())
		return nil, errors.New("error: failed to get player saving account")
	}

	result := new(player.PlayerSavingAccount)
	for cursor.Next(ctx) {
		if err := cursor.Decode(result); err != nil {
			log.Printf("Error: GetPlayerSavingAccount: %s", err.Error())
			return nil, errors.New("error: failed to get player saving account")
		}
	}

	if result.PlayerId == "" {
		result.PlayerId = playerId
	}

	return result, nil

}

func (r *playerRepository) FindOnePlayerCredential(pctx context.Context, email string) (*player.Player, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn()
	col := db.Collection("players")

	result := new(player.Player)

	log.Printf("email: %s", email)

	if err := col.FindOne(ctx, bson.M{"email": email}).Decode(result); err != nil {

		log.Printf("Error: FindOnePlayerCredential: %s", err.Error())
		return nil, errors.New("error: email is invalid")
	}

	return result, nil
}

func (r *playerRepository) FindOnePlayerProfileTokenRefresh(pctx context.Context, player_id string) (*player.Player, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn()
	col := db.Collection("players")

	result := new(player.Player)

	if err := col.FindOne(ctx, bson.M{"_id": utils.ConvertToObjectId(player_id)}).Decode(result); err != nil {

		log.Printf("Error: FindOnePlayerCredential: %s", err.Error())
		return nil, errors.New("error: player profile not found")
	}

	return result, nil
}

func (r *playerRepository) DockedPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: DockedPlayer MouneyRes failed: %s", err.Error())
		return errors.New("error: docked player money failed")
	}

	if err := queue.PushMessageWithKeyToQueue(
		[]string{cfg.Kafka.Url},
		cfg.Kafka.ApiKey,
		cfg.Kafka.Secret,
		"payment",
		"buy",
		reqInBytes,
	); err != nil {
		log.Printf("Error: DockedPlayerMoneyRes failed: %s", err.Error())
		return errors.New("error: docked player mouneyRes failed")
	}
	return nil
}

func (r *playerRepository) DeleteOnePlayerTransaction(pctx context.Context, transactionId string) error {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	objectId, err := utils.ParseObjectId(transactionId)
	if err != nil {
		return err
	}

	db := r.playerDbConn()
	col := db.Collection("player_transactions")
	walletCol := db.Collection("player_wallets")

	tx := new(player.PlayerTransaction)
	if err := col.FindOne(ctx, bson.M{"_id": objectId}).Decode(tx); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return errors.New("error: item not found")
		}
		return errors.New("error: delete one item failed")
	}

	result, err := col.DeleteOne(ctx, bson.M{"_id": objectId})
	if err != nil {
		log.Printf("Error: DeleteOneItem failed: %s", err.Error())
		return errors.New("error: delete one item failed")
	}

	if result.DeletedCount == 0 {
		return errors.New("error: item not found")
	}

	if err := r.applyWalletDelta(ctx, walletCol, tx.PlayerId, -tx.Amount); err != nil {
		log.Printf("Error: wallet sync on delete transaction %s: %s", transactionId, err.Error())
	}

	return nil
}

func (r *playerRepository) AddPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *payment.PaymentTransferRes) error {
	reqInBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error: AddPlayerMoneyRes failed: %s", err.Error())
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
		log.Printf("Error: AddPlayerMoneyRes failed: %s", err.Error())
		return errors.New("error: docked player money res failed")
	}

	return nil
}
