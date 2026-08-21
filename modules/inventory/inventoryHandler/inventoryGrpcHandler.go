package inventoryHandler

import (
	"context"

	inventoryPb "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryPb"
	inventoryusecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"
)

type (
	inventoryGrpcHandler struct {
		inventoryusecase inventoryusecase.InventoryUsecaseService
		inventoryPb.UnimplementedInventoryGrpcServiceServer
	}
)

func NewInventoryGrpcHandler(inventoryusecase inventoryusecase.InventoryUsecaseService) *inventoryGrpcHandler {
	return &inventoryGrpcHandler{
		inventoryusecase: inventoryusecase,
	}
}

func (g *inventoryGrpcHandler) IsAvailableToSell(ctx context.Context, req *inventoryPb.IsAvailableToSellRes) (*inventoryPb.IsAvailableToSellRes, error) {
	return nil, nil
}
