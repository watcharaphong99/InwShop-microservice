package playerHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"
)

type (
	PlayerQueueHandlerService interface{}

	playerQueueHandler struct {
		cfg           *config.Config
		playerUsecase playerUsecase.PlayerUsecaseService
	}
)

func NewPlayerQueueHandler(cfg *config.Config, playerUsecase playerUsecase.PlayerUsecaseService) PlayerQueueHandlerService {
	return &playerQueueHandler{cfg: cfg, playerUsecase: playerUsecase}
}
