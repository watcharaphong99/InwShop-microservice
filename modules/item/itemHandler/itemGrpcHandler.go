package itemHandler

import (
	"context"

	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"
	itemUsecase "github.com/watcharaphong99/InwzaShop/modules/item/itemUseCase"
)

type (
	itemGrpcHandler struct {
		itemPb.UnimplementedItemGrpcServiceServer
		itemUsecase itemUsecase.ItemUsecaseService
	}
)

func NewItemGrpcHandler(itemUsecase itemUsecase.ItemUsecaseService) *itemGrpcHandler {
	return &itemGrpcHandler{itemUsecase: itemUsecase}
}

func (g *itemGrpcHandler) FindItemsInIds(ctx context.Context, req *itemPb.FindItemsInIdsReq) (*itemPb.FindItemsInIdsRes, error) {
	return nil, nil
}
