package itemUsecase

import "github.com/watcharaphong99/InwzaShop/modules/item/itemRepository"

type (
	ItemUsecaseService interface{}

	itemUsecase struct {
		itemRepo itemRepository.ItemRepositoryService
	}
)

func NewItemUsecaseService(repo itemRepository.ItemRepositoryService) ItemUsecaseService {
	return &itemUsecase{itemRepo: repo}
}
