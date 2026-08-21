package server

import (
	"github.com/watcharaphong99/InwzaShop/modules/player/playerHandler"
	"github.com/watcharaphong99/InwzaShop/modules/player/playerRepository"
	playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"
)

func (s *server) playerService() {
	repo := playerRepository.NewPlayerRepository(s.db)
	usecase := playerUsecase.NewPlayerUsecase(repo)
	httpHander := playerHandler.NewPlayerHttpHandlerService(s.cfg, usecase)
	grpcHandler := playerHandler.NewPlayerGrpcHandler(usecase)
	queue := playerHandler.NewPlayerQueueHandler(s.cfg, usecase)

	_ = httpHander
	_ = usecase
	_ = grpcHandler
	_ = queue

	player := s.app.Group("/player_v1")

	//help chek
	player.GET("", s.healthcheckService)

}
