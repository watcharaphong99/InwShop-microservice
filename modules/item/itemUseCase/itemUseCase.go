package itemUsecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/watcharaphong99/InwzaShop/modules/item"
	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"
	"github.com/watcharaphong99/InwzaShop/modules/item/itemRepository"
	"github.com/watcharaphong99/InwzaShop/modules/models"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	ItemUsecaseService interface {
		CreateItem(pctx context.Context, req *item.CreateItemReq) (*item.ItemShowCase, error)
		FindOneItem(pctx context.Context, itemId string) (*item.ItemShowCase, error)
		FindManyItems(pctx context.Context, basePaginateUrl string, req *item.ItemSearchReq) (*models.PaginateRes, error)
		EditItem(pctx context.Context, itemId string, req *item.ItemUpdateReq) (*item.ItemShowCase, error)
		EnableOrDisableItem(pctx context.Context, itemId string, usageStatus bool) (bool, error)
		// FindItemsInIds(pctx context.Context, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error)
		DeleteItem(pctx context.Context, itemId string) (int64, error)
		FindItemInIds(pctx context.Context, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error)
	}

	itemUsecase struct {
		itemRepository itemRepository.ItemRepositoryService
	}
)

func NewItemUsecaseService(repo itemRepository.ItemRepositoryService) ItemUsecaseService {
	return &itemUsecase{itemRepository: repo}
}

func (u *itemUsecase) CreateItem(pctx context.Context, req *item.CreateItemReq) (*item.ItemShowCase, error) {
	isUnique, err := u.itemRepository.IsUniqueItem(pctx, req.Title)
	if err != nil {
		return nil, err
	}
	if !isUnique {
		return nil, errors.New("error: this title is already exist")
	}

	itemId, err := u.itemRepository.InsertOneItem(pctx, &item.Item{
		Title:       req.Title,
		Price:       req.Price,
		Damage:      req.Damage,
		UsageStatus: true,
		ImageUrl:    req.ImageUrl,
		CreatedAt:   utils.LocalTime(),
		UpdatedAt:   utils.LocalTime(),
	})
	if err != nil {
		return nil, err
	}

	return u.FindOneItem(pctx, itemId.Hex())
}

func (u *itemUsecase) FindOneItem(pctx context.Context, itemId string) (*item.ItemShowCase, error) {
	result, err := u.itemRepository.FindOneItem(pctx, itemId)
	if err != nil {
		return nil, err
	}

	return &item.ItemShowCase{
		ItemId:   "item:" + result.Id.Hex(),
		Title:    result.Title,
		Price:    result.Price,
		Damage:   result.Damage,
		ImageUrl: result.ImageUrl,
	}, nil
}

func (u *itemUsecase) FindManyItems(pctx context.Context, basePaginateUrl string, req *item.ItemSearchReq) (*models.PaginateRes, error) {
	findItemsFilter := bson.D{}
	findItemsOpts := make([]*options.FindOptions, 0)

	countItemsFilter := bson.D{}

	//filter
	if req.Start != "" {
		startId, err := utils.ParseObjectId(strings.TrimPrefix(req.Start, "item:"))
		if err != nil {
			return nil, err
		}
		findItemsFilter = append(findItemsFilter, bson.E{Key: "_id", Value: bson.D{{Key: "$gt", Value: startId}}})
	}

	if req.Title != "" {
		titleRegex := primitive.Regex{Pattern: regexp.QuoteMeta(req.Title), Options: "i"}
		findItemsFilter = append(findItemsFilter, bson.E{Key: "title", Value: titleRegex})
		countItemsFilter = append(countItemsFilter, bson.E{Key: "title", Value: titleRegex})
	}

	findItemsFilter = append(findItemsFilter, bson.E{Key: "usage_status", Value: true})
	countItemsFilter = append(countItemsFilter, bson.E{Key: "usage_status", Value: true})

	//Options
	findItemsOpts = append(findItemsOpts, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	findItemsOpts = append(findItemsOpts, options.Find().SetLimit(int64(req.Limit)))

	var (
		results []*item.ItemShowCase
		err     error
	)
	if req.Title == "" && req.Start == "" {
		results, err = u.itemRepository.FindActiveItemsPage(pctx, req.Limit)
	} else {
		results, err = u.itemRepository.FindManyItems(pctx, findItemsFilter, findItemsOpts)
	}
	if err != nil {
		return nil, err
	}

	var total int64
	if req.Title == "" {
		total, err = u.itemRepository.CountActiveItems(pctx)
	} else {
		total, err = u.itemRepository.CountItems(pctx, countItemsFilter)
	}
	if err != nil {
		return nil, err
	}

	res := &models.PaginateRes{
		Data:  results,
		Total: total,
		Limit: req.Limit,
		First: models.FirstPaginate{
			Href: fmt.Sprintf("%s?limit=%d&title=%s", basePaginateUrl, req.Limit, url.QueryEscape(req.Title)),
		},
	}

	// if len(results) < req.Limit {
	// 	return res, nil
	// }

	if len(results) == 0 || len(results) < req.Limit {
		return res, nil
	}

	lastId := results[len(results)-1].ItemId
	res.Next = models.NextPaginate{
		Start: lastId,
		Href:  fmt.Sprintf("%s?limit=%d&title=%s&start=%s", basePaginateUrl, req.Limit, url.QueryEscape(req.Title), lastId),
	}

	return res, nil
}

func (u *itemUsecase) EditItem(pctx context.Context, itemId string, req *item.ItemUpdateReq) (*item.ItemShowCase, error) {
	current, err := u.itemRepository.FindOneItem(pctx, itemId)
	if err != nil {
		return nil, err
	}

	updateReq := bson.M{}

	if req.Title != "" && req.Title != current.Title {
		isUnique, err := u.itemRepository.IsUniqueItem(pctx, req.Title)
		if err != nil {
			return nil, err
		}
		if !isUnique {
			log.Println("Error: EditItem failed: title is already exist")
			return nil, errors.New("error: this title is already exist")
		}

		updateReq["title"] = req.Title
	}

	if req.ImageUrl != "" {
		updateReq["image_url"] = req.ImageUrl
	}

	if req.Damage != nil {
		updateReq["damage"] = *req.Damage
	}

	if req.Price != nil {
		updateReq["price"] = *req.Price
	}

	if len(updateReq) == 0 {
		return nil, errors.New("error: no fields to update")
	}

	updateReq["updated_at"] = utils.LocalTime()

	if err := u.itemRepository.UpdateOneItem(pctx, itemId, updateReq); err != nil {
		return nil, err
	}

	return u.FindOneItem(pctx, itemId)
}

func (u *itemUsecase) EnableOrDisableItem(pctx context.Context, itemId string, usageStatus bool) (bool, error) {
	if err := u.itemRepository.EnableOrDisableItem(pctx, itemId, usageStatus); err != nil {
		return false, err
	}

	return usageStatus, nil
}

// func (u *itemUsecase) FindItemsInIds(pctx context.Context, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
// 	objectIds := make([]primitive.ObjectID, 0, len(req.Ids))
// 	for _, id := range req.Ids {
// 		objectId, err := utils.ParseObjectId(strings.TrimPrefix(id, "item:"))
// 		if err != nil {
// 			return nil, err
// 		}
// 		objectIds = append(objectIds, objectId)
// 	}

// 	results, err := u.itemRepository.FindItemsInIds(pctx, objectIds)
// 	if err != nil {
// 		return nil, err
// 	}

// 	items := make([]*itemPb.Item, 0, len(results))
// 	for _, result := range results {
// 		items = append(items, &itemPb.Item{
// 			Id:       "item:" + result.Id.Hex(),
// 			Title:    result.Title,
// 			Price:    result.Price,
// 			ImageUrl: result.ImageUrl,
// 			Damage:   int32(result.Damage),
// 		})
// 	}

// 	return &itemPb.FindItemsInIdsRes{Items: items}, nil
// }

func (u *itemUsecase) DeleteItem(pctx context.Context, itemId string) (int64, error) {

	result, err := u.itemRepository.DeleteOneItem(pctx, itemId)
	if err != nil {
		return 0, err
	}

	return result, nil
}

// todo
func (u *itemUsecase) FindItemInIds(pctx context.Context, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
	filter := bson.D{}

	objectIds := make([]primitive.ObjectID, 0)
	for _, itemId := range req.Ids {
		objectIds = append(objectIds, utils.ConvertToObjectId(strings.TrimPrefix(itemId, "item:")))
	}

	filter = append(filter, bson.E{Key: "_id", Value: bson.D{{Key: "$in", Value: objectIds}}})
	filter = append(filter, bson.E{Key: "usage_status", Value: true})

	results, err := u.itemRepository.FindManyItems(pctx, filter, nil)
	if err != nil {
		return nil, err
	}

	resultsToRes := make([]*itemPb.Item, 0)
	for _, result := range results {
		resultsToRes = append(resultsToRes, &itemPb.Item{
			Id:       result.ItemId,
			Title:    result.Title,
			Price:    result.Price,
			Damage:   int32(result.Damage),
			ImageUrl: result.ImageUrl,
		})
	}

	return &itemPb.FindItemsInIdsRes{
		Items: resultsToRes,
	}, nil
}
