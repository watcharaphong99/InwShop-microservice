package migration

import (
	"context"
	"log"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/player"
	"github.com/watcharaphong99/InwzaShop/pkg/database"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func playerDbConn(pctx context.Context, cfg *config.Config) *mongo.Database {
	return database.DbConn(pctx, cfg).Database("player_db")
}

func PlayerMigrate(pctx context.Context, cfg *config.Config) {
	db := playerDbConn(pctx, cfg)
	defer db.Client().Disconnect(pctx)

	col := db.Collection("player_transactions")
	mustCreateIndexes(pctx, col, []mongo.IndexModel{
		{Keys: bson.D{{"_id", 1}}},
		{Keys: bson.D{{"player_id", 1}}},
		{
			Keys:    bson.D{{"event_id", 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	})

	col = db.Collection("players")
	dropIndexIfExists(pctx, col, "email_1")
	mustCreateIndexes(pctx, col, []mongo.IndexModel{
		{Keys: bson.D{{"_id", 1}}},
		{Keys: bson.D{{"email", 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{"username", 1}}, Options: options.Index().SetUnique(true)},
	})

	documents := func() []any {
		roles := []*player.Player{
			{
				Email: "player001@Iwpshop.com",
				Password: func() string {
					// Hashing password
					hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
					return string(hashedPassword)
				}(),
				Username: "Player001",
				PlayerRoles: []player.PlayerRole{
					{
						RoleTitle: "player",
						RoleCode:  0,
					},
				},
				CreatedAt: utils.LocalTime(),
				UpdatedAt: utils.LocalTime(),
			},
			{
				Email: "player002@Iwpshop.com",
				Password: func() string {
					// Hashing password
					hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
					return string(hashedPassword)
				}(),
				Username: "Player002",
				PlayerRoles: []player.PlayerRole{
					{
						RoleTitle: "player",
						RoleCode:  0,
					},
				},
				CreatedAt: utils.LocalTime(),
				UpdatedAt: utils.LocalTime(),
			},
			{
				Email: "player003@Iwpshop.com",
				Password: func() string {
					// Hashing password
					hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
					return string(hashedPassword)
				}(),
				Username: "Player003",
				PlayerRoles: []player.PlayerRole{
					{
						RoleTitle: "player",
						RoleCode:  0,
					},
				},
				CreatedAt: utils.LocalTime(),
				UpdatedAt: utils.LocalTime(),
			},
			{
				Email: "admin001@Iwpshop.com",
				Password: func() string {
					// Hashing password
					hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
					return string(hashedPassword)
				}(),
				Username: "Admin001",
				PlayerRoles: []player.PlayerRole{
					{
						RoleTitle: "player",
						RoleCode:  0,
					},
					{
						RoleTitle: "admin",
						RoleCode:  1,
					},
				},
				CreatedAt: utils.LocalTime(),
				UpdatedAt: utils.LocalTime(),
			},
		}

		docs := make([]any, 0)
		for _, r := range roles {
			docs = append(docs, r)
		}
		return docs
	}()

	results, err := col.InsertMany(pctx, documents, nil)
	if err != nil {
		panic(err)
	}
	log.Println("Migrate auth completed: ", results)

	playerTransactions := make([]any, 0)
	for _, p := range results.InsertedIDs {
		playerTransactions = append(playerTransactions, &player.PlayerTransaction{
			PlayerId:  "player:" + p.(primitive.ObjectID).Hex(),
			Amount:    1000,
			CreatedAt: utils.LocalTime(),
		})
	}

	col = db.Collection("player_transactions")
	results, err = col.InsertMany(pctx, playerTransactions, nil)

	if err != nil {
		panic(err)
	}
	log.Println("Migrate player_transactions completed: ", results)

	col = db.Collection("player_transactions_queue")
	seedKafkaOffset(pctx, col)
}
