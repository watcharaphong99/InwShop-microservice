package server

import (
	"github.com/watcharaphong99/InwzaShop/modules/player/playerHandler"
	"github.com/watcharaphong99/InwzaShop/modules/player/playerRepository"
	playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"
)

func (s *server) playerService() {
	repo := playerRepository.NewPlayerRepositoryService(s.db)
	usecase := playerUsecase.NewPlayerUsecaseService(repo)
	httpHander := playerHandler.NewPlayerHttpHandlerService(s.cfg, usecase)
	grpcHandler := playerHandler.NewPlayerGrpcHandlerService(usecase)
	queue := playerHandler.NewPlayerQueueHandlerService(s.cfg, usecase)

	_ = httpHander
	_ = usecase
	_ = grpcHandler
	_ = queue

	player := s.app.Group("/player_v1")

	//help chek
	player.GET("", s.healthcheckService)

}
