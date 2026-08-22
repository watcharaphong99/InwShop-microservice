package server

import (
	"log"

	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"

	"github.com/watcharaphong99/InwzaShop/modules/item/itemHandler"
	"github.com/watcharaphong99/InwzaShop/modules/item/itemRepository"
	itemUsecase "github.com/watcharaphong99/InwzaShop/modules/item/itemUseCase"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
)

func (s *server) itemService() {
	repo := itemRepository.NewItemRepository(s.db)
	usecase := itemUsecase.NewItemUsecaseService(repo)
	httpHandler := itemHandler.NewItemHttpHandler(s.cfg, usecase)
	grpcHandler := itemHandler.NewItemGrpcHandler(usecase)

	//gRpc
	go func() {
		grpcServer, lis := grpccon.NewGrpcServer(&s.cfg.Jwt, s.cfg.Grpc.ItemUrl)

		itemPb.RegisterItemGrpcServiceServer(grpcServer, grpcHandler)
		log.Printf("Item gRPC server listening on %s", s.cfg.Grpc.ItemUrl)
		grpcServer.Serve(lis)
	}()

	_ = httpHandler
	_ = grpcHandler

	item := s.app.Group("/item_v1")

	//help check
	item.GET("", s.healthcheckService)

}
