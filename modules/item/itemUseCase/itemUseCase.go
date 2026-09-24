package itemUsecase

import (
	"context"
	"errors"

	"github.com/watcharaphong99/InwzaShop/modules/item"
	"github.com/watcharaphong99/InwzaShop/modules/item/itemRepository"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
)

type (
	ItemUsecaseService interface {
		CreateItem(pctx context.Context, req *item.CreateItemReq) (*item.ItemShowCase, error)
		FindOneItem(pctx context.Context, itemId string) (*item.ItemShowCase, error)
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
