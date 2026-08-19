package server

import (
	"github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryHandler"
	"github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryRepository"
	inventoryusecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"
)

func (s *server) inventoryService() {
	repo := inventoryRepository.NewInventoryRepository(s.db)
	usecase := inventoryusecase.NewInventoryUsecase(repo)
	httpHandler := inventoryHandler.NewInventoryHttpHandler(s.cfg, usecase)
	grpcHandler := inventoryHandler.NewInventoryGrpcHandler(usecase)

	_ = httpHandler
	_ = grpcHandler

	inventory := s.app.Group("/invenroty_v1")

	//help check
	inventory.GET("", s.healthcheckService)

}
