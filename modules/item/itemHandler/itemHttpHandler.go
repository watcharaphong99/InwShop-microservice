package itemHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	itemUsecase "github.com/watcharaphong99/InwzaShop/modules/item/itemUseCase"
)

type (
	ItemHttpHandlerService interface{}

	itemHttpHandler struct {
		cfg         *config.Config
		itemUsecase itemUsecase.ItemUsecaseService
	}
)

func NewItemHttpHandler(cfg *config.Config, itemUsecase itemUsecase.ItemUsecaseService) ItemHttpHandlerService {
	return &itemHttpHandler{cfg: cfg, itemUsecase: itemUsecase}
}
