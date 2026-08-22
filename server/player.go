package server

import (
	"log"

	"github.com/watcharaphong99/InwzaShop/modules/player/playerHandler"
	PlayerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/modules/player/playerRepository"
	playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
)

func (s *server) playerService() {
	repo := playerRepository.NewPlayerRepository(s.db)
	usecase := playerUsecase.NewPlayerUsecase(repo)
	httpHander := playerHandler.NewPlayerHttpHandlerService(s.cfg, usecase)
	grpcHandler := playerHandler.NewPlayerGrpcHandler(usecase)
	queue := playerHandler.NewPlayerQueueHandler(s.cfg, usecase)

	// //gRpc
	go func() {
		grpcServer, lis := grpccon.NewGrpcServer(&s.cfg.Jwt, s.cfg.Grpc.PlayerUrl)

		PlayerPb.RegisterPlayerGrpcServiceServer(grpcServer, grpcHandler)
		log.Printf("Player gRPC server listening on %s", s.cfg.Grpc.PlayerUrl)
		grpcServer.Serve(lis)
	}()

	_ = httpHander
	_ = usecase
	_ = grpcHandler
	_ = queue

	player := s.app.Group("/player_v1")

	//help chek
	player.GET("", s.healthcheckService)

}
