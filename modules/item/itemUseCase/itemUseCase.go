package itemUsecase

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/watcharaphong99/InwzaShop/modules/item"
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
		findItemsFilter = append(findItemsFilter, bson.E{"_id", bson.D{{"$gt", startId}}})
	}

	if req.Title != "" {
		titleRegex := primitive.Regex{Pattern: regexp.QuoteMeta(req.Title), Options: "i"}
		findItemsFilter = append(findItemsFilter, bson.E{"title", titleRegex})
		countItemsFilter = append(countItemsFilter, bson.E{"title", titleRegex})
	}

	findItemsFilter = append(findItemsFilter, bson.E{"usage_status", true})
	countItemsFilter = append(countItemsFilter, bson.E{"usage_status", true})

	//Options
	findItemsOpts = append(findItemsOpts, options.Find().SetSort(bson.D{{"_id", 1}}))
	findItemsOpts = append(findItemsOpts, options.Find().SetLimit(int64(req.Limit)))

	//Find
	results, err := u.itemRepository.FindManyItems(pctx, findItemsFilter, findItemsOpts)
	if err != nil {
		return nil, err
	}

	//Count
	total, err := u.itemRepository.CountItems(pctx, countItemsFilter)
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
