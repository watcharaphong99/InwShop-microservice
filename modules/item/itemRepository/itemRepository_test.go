package itemRepository

import (
	"context"
	"testing"

	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestDeleteOneItem(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("deletes the item", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int64(1)}))

		id := primitive.NewObjectID()
		repo := &itemRepository{db: mt.Client, cache: &rediscon.Client{}}

		got, err := repo.DeleteOneItem(context.Background(), id.Hex())
		if err != nil {
			mt.Fatalf("unexpected error: %v", err)
		}
		if got != 1 {
			mt.Fatalf("deleted count = %d, want 1", got)
		}

		evt := deleteStartedEvent(mt)
		if evt == nil {
			mt.Fatal("expected a delete command")
		}
		if db := evt.Command.Lookup("delete"); db.StringValue() != "items" {
			mt.Fatalf("collection = %q, want items", db.StringValue())
		}

		deletes, err := evt.Command.Lookup("deletes").Array().Values()
		if err != nil || len(deletes) != 1 {
			mt.Fatalf("deletes = %v, err %v", deletes, err)
		}
		var op struct {
			Q     bson.M `bson:"q"`
			Limit int32  `bson:"limit"`
		}
		if err := bson.Unmarshal(deletes[0].Value, &op); err != nil {
			mt.Fatalf("unmarshal delete op: %v", err)
		}
		if op.Limit != 1 {
			mt.Fatalf("limit = %d, want 1", op.Limit)
		}
		gotID, ok := op.Q["_id"].(primitive.ObjectID)
		if !ok || gotID != id {
			mt.Fatalf("filter _id = %v, want %s", op.Q["_id"], id.Hex())
		}
	})

	mt.Run("item not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: int64(0)}))

		repo := &itemRepository{db: mt.Client, cache: &rediscon.Client{}}
		got, err := repo.DeleteOneItem(context.Background(), primitive.NewObjectID().Hex())
		if err == nil || err.Error() != "error: item not found" {
			mt.Fatalf("count %d, err %v", got, err)
		}
		if got != 0 {
			mt.Fatalf("deleted count = %d, want 0", got)
		}
	})

	mt.Run("delete command fails", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "boom",
		}))

		repo := &itemRepository{db: mt.Client, cache: &rediscon.Client{}}
		got, err := repo.DeleteOneItem(context.Background(), primitive.NewObjectID().Hex())
		if err == nil || err.Error() != "error: delete one item failed" {
			mt.Fatalf("count %d, err %v", got, err)
		}
		if got != 0 {
			mt.Fatalf("deleted count = %d, want 0", got)
		}
	})
}

func TestDeleteOneItemInvalidId(t *testing.T) {
	repo := &itemRepository{cache: &rediscon.Client{}}

	got, err := repo.DeleteOneItem(context.Background(), "not-an-object-id")
	if err == nil || err.Error() != "error: id is invalid" {
		t.Fatalf("count %d, err %v", got, err)
	}
	if got != 0 {
		t.Fatalf("deleted count = %d, want 0", got)
	}
}

func deleteStartedEvent(mt *mtest.T) *event.CommandStartedEvent {
	mt.Helper()
	for {
		evt := mt.GetStartedEvent()
		if evt == nil {
			return nil
		}
		if evt.CommandName == "delete" {
			return evt
		}
	}
}
