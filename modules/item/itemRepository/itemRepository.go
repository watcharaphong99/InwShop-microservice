package itemRepository

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/watcharaphong99/InwzaShop/modules/item"
	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	ItemRepositoryService interface {
		IsUniqueItem(pctx context.Context, title string) (bool, error)
		InsertOneItem(pctx context.Context, req *item.Item) (primitive.ObjectID, error)
		FindOneItem(pctx context.Context, itemId string) (*item.Item, error)
		FindItemsInIds(pctx context.Context, objectIds []primitive.ObjectID) ([]*item.Item, error)
		CountItems(pctx context.Context, filter primitive.D) (int64, error)
		FindManyItems(pctx context.Context, filter primitive.D, opts []*options.FindOptions) ([]*item.ItemShowCase, error)
		UpdateOneItem(pctx context.Context, itemId string, req primitive.M) error
		EnableOrDisableItem(pctx context.Context, itemId string, isActive bool) error
		DeleteOneItem(pctx context.Context, itemId string) (int64, error)
	}

	itemRepository struct {
		db    *mongo.Client
		cache *rediscon.Client
	}
)

func NewItemRepository(db *mongo.Client, cache *rediscon.Client) ItemRepositoryService {
	return &itemRepository{db: db, cache: cache}
}

func (r *itemRepository) itemDbConn(pctx context.Context) *mongo.Database {
	return r.db.Database("item_db")
}

func (r *itemRepository) IsUniqueItem(pctx context.Context, title string) (bool, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	result := new(item.Item)
	if err := col.FindOne(ctx, bson.M{"title": title}).Decode(result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return true, nil
		}
		log.Printf("Error: IsUniqueItem: %s", err.Error())
		return false, errors.New("error: check unique item failed")
	}
	return false, nil
}

func (r *itemRepository) InsertOneItem(pctx context.Context, req *item.Item) (primitive.ObjectID, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	itemId, err := col.InsertOne(ctx, req)
	if err != nil {
		log.Printf("Error: InsertOneItem: %s", err.Error())
		return primitive.NilObjectID, errors.New("error: insert one item failed")
	}

	return itemId.InsertedID.(primitive.ObjectID), nil
}

func (r *itemRepository) FindOneItem(pctx context.Context, itemId string) (*item.Item, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	objectId, err := utils.ParseObjectId(itemId)
	if err != nil {
		return nil, err
	}

	cacheKey := rediscon.ItemKey(objectId.Hex())
	cached := new(item.Item)
	if r.cache.GetJSON(ctx, cacheKey, cached) {
		return cached, nil
	}

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	result := new(item.Item)
	if err := col.FindOne(ctx, bson.M{"_id": objectId}).Decode(result); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("error: item not found")
		}
		log.Printf("Error: FindOneItem failed: %s", err.Error())
		return nil, errors.New("error: find one item failed")
	}

	r.cache.SetJSON(ctx, cacheKey, result, rediscon.ItemTTL)

	return result, nil
}

// FindItemsInIds returns items in the same order as objectIds; ids that do not exist are skipped.
func (r *itemRepository) FindItemsInIds(pctx context.Context, objectIds []primitive.ObjectID) ([]*item.Item, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	if len(objectIds) == 0 {
		return make([]*item.Item, 0), nil
	}

	keys := make([]string, len(objectIds))
	for i, id := range objectIds {
		keys[i] = rediscon.ItemKey(id.Hex())
	}

	found := make(map[primitive.ObjectID]*item.Item, len(objectIds))
	missIds := make([]primitive.ObjectID, 0)

	for i, raw := range r.cache.MGetBytes(ctx, keys...) {
		if raw != nil {
			cached := new(item.Item)
			if err := json.Unmarshal(raw, cached); err == nil {
				found[objectIds[i]] = cached
				continue
			}
		}
		missIds = append(missIds, objectIds[i])
	}

	if len(missIds) > 0 {
		col := r.itemDbConn(ctx).Collection("items")

		cursors, err := col.Find(ctx, bson.M{"_id": bson.M{"$in": missIds}})
		if err != nil {
			log.Printf("Error: FindItemsInIds failed: %s", err.Error())
			return nil, errors.New("error: find items in ids failed")
		}
		defer cursors.Close(ctx)

		toCache := make(map[string]any, len(missIds))
		for cursors.Next(ctx) {
			result := new(item.Item)
			if err := cursors.Decode(result); err != nil {
				log.Printf("Error: FindItemsInIds failed: %s", err.Error())
				return nil, errors.New("error: find items in ids failed")
			}
			found[result.Id] = result
			toCache[rediscon.ItemKey(result.Id.Hex())] = result
		}
		if err := cursors.Err(); err != nil {
			log.Printf("Error: FindItemsInIds failed: %s", err.Error())
			return nil, errors.New("error: find items in ids failed")
		}

		r.cache.SetManyJSON(ctx, toCache, rediscon.ItemTTL)
	}

	results := make([]*item.Item, 0, len(found))
	for _, id := range objectIds {
		if it, ok := found[id]; ok {
			results = append(results, it)
		}
	}
	return results, nil
}

func (r *itemRepository) FindManyItems(pctx context.Context, filter primitive.D, opts []*options.FindOptions) ([]*item.ItemShowCase, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	cursors, err := col.Find(ctx, filter, opts...)
	if err != nil {
		log.Printf("Error: FindManyItems failed: %s", err.Error())
		return make([]*item.ItemShowCase, 0), errors.New("error: find many items failed")
	}
	defer cursors.Close(ctx)

	results := make([]*item.ItemShowCase, 0)

	for cursors.Next(ctx) {
		result := new(item.Item)

		if err := cursors.Decode(result); err != nil {
			log.Printf("Error: FindManyItems failed: %s", err.Error())
			return make([]*item.ItemShowCase, 0), errors.New("error: find many items failed")
		}

		results = append(results, &item.ItemShowCase{
			ItemId:   "item:" + result.Id.Hex(),
			Title:    result.Title,
			Price:    result.Price,
			Damage:   result.Damage,
			ImageUrl: result.ImageUrl,
		})
	}

	if err := cursors.Err(); err != nil {
		log.Printf("Error: FindManyItems failed: %s", err.Error())
		return make([]*item.ItemShowCase, 0), errors.New("error: find many items failed")
	}

	return results, nil
}

func (r *itemRepository) CountItems(pctx context.Context, filter primitive.D) (int64, error) {

	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	count, err := col.CountDocuments(ctx, filter)
	if err != nil {
		log.Printf("Error: CountItems failed: %s", err.Error())
		return -1, errors.New("error: count items failed")
	}

	return count, nil
}

func (r *itemRepository) UpdateOneItem(pctx context.Context, itemId string, req primitive.M) error {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	objectId, err := utils.ParseObjectId(itemId)
	if err != nil {
		return err
	}

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	// result, err := col.UpdateOne(ctx, bson.M{"_id": utils.ConvertToObjectId(itemId)}, bson.M{"$set": req})
	result, err := col.UpdateOne(ctx, bson.M{"_id": objectId}, bson.M{"$set": req})
	if err != nil {
		log.Printf("Error: UpdateOneItem failed: %s", err.Error())
		return errors.New("error: update one item failed")
	}
	if result.MatchedCount == 0 {
		log.Printf("Error: UpdateOneItem failed: item_id %s not found", itemId)
		return errors.New("error: item not found")
	}
	log.Printf("UpdateOneItem result: %v", result.ModifiedCount)

	r.cache.Del(ctx, rediscon.ItemKey(objectId.Hex()))

	return nil
}

func (r *itemRepository) EnableOrDisableItem(pctx context.Context, itemId string, isActive bool) error {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	objectId, err := utils.ParseObjectId(itemId)
	if err != nil {
		return err
	}

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	// result, err := col.UpdateOne(ctx, bson.M{"_id": utils.ConvertToObjectId(itemId)}, bson.M{"$set": bson.M{"usage_status": isActive}})
	result, err := col.UpdateOne(ctx, bson.M{"_id": objectId}, bson.M{"$set": bson.M{
		"usage_status": isActive,
		"updated_at":   utils.LocalTime(),
	}})

	if err != nil {
		log.Printf("Error: EnableOrDisableItem failed: %s", err.Error())
		return errors.New("error: enable or disable item failed")
	}
	if result.MatchedCount == 0 {
		log.Printf("Error: EnableOrDisableItem failed: item_id %s not found", itemId)
		return errors.New("error: item not found")
	}
	log.Printf("EnableOrDisableItem result: %v", result.ModifiedCount)

	r.cache.Del(ctx, rediscon.ItemKey(objectId.Hex()))

	return nil
}

func (r *itemRepository) DeleteOneItem(pctx context.Context, itemId string) (int64, error) {
	ctx, cancle := context.WithTimeout(pctx, 10*time.Second)
	defer cancle()

	objectId, err := utils.ParseObjectId(itemId)
	if err != nil {
		return 0, err
	}

	db := r.itemDbConn(ctx)
	col := db.Collection("items")

	result, err := col.DeleteOne(ctx, bson.M{"_id": objectId})
	if err != nil {
		log.Printf("Error: DeleteOneItem failed: %s", err.Error())
		return 0, errors.New("error: delete one item failed")
	}

	if result.DeletedCount == 0 {
		return 0, errors.New("error: item not found")
	}

	r.cache.Del(ctx, rediscon.ItemKey(objectId.Hex()))

	return result.DeletedCount, nil
}
