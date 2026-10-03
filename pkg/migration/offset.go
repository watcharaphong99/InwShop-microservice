package migration

import (
	"context"
	"log"

	"github.com/watcharaphong99/InwzaShop/modules/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func seedKafkaOffset(pctx context.Context, col *mongo.Collection) {
	result, err := col.UpdateOne(
		pctx,
		bson.M{"_id": models.KafkaOffsetID},
		bson.M{"$setOnInsert": bson.M{"offset": int64(-1)}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		panic(err)
	}
	log.Printf("Migrate %s offset completed: %v", col.Name(), result)
}

func mustCreateIndexes(pctx context.Context, col *mongo.Collection, idxs []mongo.IndexModel) {
	indexs, err := col.Indexes().CreateMany(pctx, idxs)
	if err != nil {
		log.Printf("Error: create indexes on %s: %s", col.Name(), err)
		panic(err)
	}
	for _, index := range indexs {
		log.Printf("Index %s: %s", col.Name(), index)
	}
}

func dropIndexIfExists(pctx context.Context, col *mongo.Collection, name string) {
	if _, err := col.Indexes().DropOne(pctx, name); err != nil {
		log.Printf("Info: drop index %s on %s: %s", name, col.Name(), err)
	}
}
