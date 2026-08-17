package inventoryHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	inventoryusecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"
)

type (
	InventoryQueueHandlerService interface{}

	inventoryQueueHandler struct {
		cfg              *config.Config
		inventoryUsecase inventoryusecase.InventoryUsecaseService
	}
)

func NewInventoryQueueHandler(cfg *config.Config, inventoryUsecase inventoryusecase.InventoryUsecaseService) InventoryQueueHandlerService {
	return &inventoryQueueHandler{
		cfg:              cfg,
		inventoryUsecase: inventoryUsecase,
	}
}
