package server

import (
	"github.com/watcharaphong99/InwzaShop/modules/item/itemHandler"
	"github.com/watcharaphong99/InwzaShop/modules/item/itemRepository"
	itemUsecase "github.com/watcharaphong99/InwzaShop/modules/item/itemUseCase"
)

func (s *server) itemService() {
	repo := itemRepository.NewItemRepository(s.db)
	usecase := itemUsecase.NewItemUsecaseService(repo)
	httpHandler := itemHandler.NewItemHttpHandler(s.cfg, usecase)
	grpcHanler := itemHandler.NewItemGrpcHandler(usecase)

	_ = httpHandler
	_ = grpcHanler

	item := s.app.Group("/item_v1")

	//help check
	_ = item

}
