package inventoryHandler

import inventoryusecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"

type (
	inventoryGrpcHandler struct {
		inventoryusecase inventoryusecase.InventoryUsecaseService
	}
)

func NewInventoryGrpcHandler(inventoryusecase inventoryusecase.InventoryUsecaseService) *inventoryGrpcHandler {
	return &inventoryGrpcHandler{
		inventoryusecase: inventoryusecase,
	}
}
