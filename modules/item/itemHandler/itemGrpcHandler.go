package itemHandler

import (
	itemUsecase "github.com/watcharaphong99/InwzaShop/modules/item/itemUseCase"
)

type (
	itemGrpcHandler struct {
		itemUsecase itemUsecase.ItemUsecaseService
	}
)

func NewItemGrpcHandler(itemUsecase itemUsecase.ItemUsecaseService) *itemGrpcHandler {
	return &itemGrpcHandler{itemUsecase: itemUsecase}
}
