package server

import (
	"log"

	"github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryHandler"
	inventoryPb "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryPb"
	"github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryRepository"
	inventoryusecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
)

func (s *server) inventoryService() {
	repo := inventoryRepository.NewInventoryRepository(s.db)
	usecase := inventoryusecase.NewInventoryUsecase(repo)
	httpHandler := inventoryHandler.NewInventoryHttpHandler(s.cfg, usecase)
	grpcHandler := inventoryHandler.NewInventoryGrpcHandler(usecase)

	//gRpc
	go func() {
		grpcServer, lis := grpccon.NewGrpcServer(&s.cfg.Jwt, s.cfg.Grpc.InventoryUrl)

		inventoryPb.RegisterInventoryGrpcServiceServer(grpcServer, grpcHandler)
		log.Printf("Inventory gRPC server listening on %s", s.cfg.Grpc.InventoryUrl)
		grpcServer.Serve(lis)
	}()

	_ = httpHandler
	_ = grpcHandler

	inventory := s.app.Group("/invenroty_v1")

	//help check
	inventory.GET("", s.healthcheckService)

}
