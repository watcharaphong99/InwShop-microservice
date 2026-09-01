package playerRepository

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/watcharaphong99/InwzaShop/modules/player"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	PlayerRepositoryService interface {
		IsUniquePlayer(pctx context.Context, email, username string) bool
		InsertOnePlayer(pctx context.Context, req *player.Player) (primitive.ObjectID, error)
		FindOnePlayerProfine(pctx context.Context, playerId string) (*player.PlayerProfileBson, error)
		InsertOnePlayerTranscation(pctx context.Context, req *player.PlayerTransaction) error
		GetPlayerSavingAccount(pctx context.Context, playerId string) (*player.PlayerSavingAccount, error)
	}

	playerRepository struct {
		db *mongo.Client
	}
)

func NewPlayerRepository(db *mongo.Client) PlayerRepositoryService {
	return &playerRepository{db: db}
}

func (r *playerRepository) playerDbConn(pctx context.Context) *mongo.Database {
	return r.db.Database("player_db")
}

func (r *playerRepository) IsUniquePlayer(pctx context.Context, email, username string) bool {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn(ctx)
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

	db := r.playerDbConn(ctx)
	col := db.Collection("players")

	playerId, err := col.InsertOne(ctx, req)
	if err != nil {
		log.Printf("Error: InsertOnePlayer: %s", err.Error())
		return primitive.NewObjectID(), errors.New("error: insert one player failed")
	}

	return playerId.InsertedID.(primitive.ObjectID), nil

}

func (r *playerRepository) FindOnePlayerProfine(pctx context.Context, playerId string) (*player.PlayerProfileBson, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn(ctx)
	col := db.Collection("players")

	result := new(player.PlayerProfileBson)

	log.Println("playerId", playerId)

	if err := col.FindOne(
		ctx,
		bson.M{"_id": utils.ConvertToObjectId(playerId)},
		options.FindOne().SetProjection(
			bson.M{
				"_id":       1,
				"email":     1,
				"username":  1,
				"create_at": 1,
				"update_at": 1,
			},
		),
	).Decode(result); err != nil {
		log.Printf("Error: FindOnePlayerProfile: %s", err.Error())
		return nil, errors.New("error: player profile not found")
	}

	return result, nil

}

func (r *playerRepository) InsertOnePlayerTranscation(pctx context.Context, req *player.PlayerTransaction) error {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn(ctx)
	col := db.Collection("player_transactions")

	result, err := col.InsertOne(ctx, req)
	if err != nil {
		log.Printf("Error: InseartOnePlayerTransaction: %s", err.Error())
		return errors.New("error: insert one player transaction failed")
	}
	log.Printf("Result: InseartOnePlayerTransaction: %v", result.InsertedID)

	return nil
}

func (r *playerRepository) GetPlayerSavingAccount(pctx context.Context, playerId string) (*player.PlayerSavingAccount, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.playerDbConn(ctx)
	col := db.Collection("player_transactions")

	log.Printf("playerId at Repository", playerId)

	filter := bson.A{
		bson.D{{"$match", bson.D{{"player_id", playerId}}}},
		bson.D{
			{"$group",
				bson.D{
					{"_id", "$player_id"},
					{"balance", bson.D{{"$sum", "$amount"}}},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"player_id", "$_id"},
					{"_id", 0},
					{"balance", 1},
				},
			},
		},
	}

	cursor, err := col.Aggregate(ctx, filter)

	log.Printf("print cursor", cursor)
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
