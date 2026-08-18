package inventoryHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	inventoryusecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"
)

type (
	InventoryHttpHandlerService interface{}

	inventoryhttpHandler struct {
		cfg              *config.Config
		inventoryusecase inventoryusecase.InventoryUsecaseService
	}
)

func NewInventoryHttpHandler(cfg *config.Config, inventoryusecase inventoryusecase.InventoryUsecaseService) InventoryHttpHandlerService {
	return inventoryhttpHandler{
		cfg:              cfg,
		inventoryusecase: inventoryusecase,
	}
}
