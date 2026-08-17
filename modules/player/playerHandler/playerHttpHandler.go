package playerHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"
)

type (
	PlayerHttpHandlerService interface{}

	playerHttpHandler struct {
		cfg           *config.Config
		playerUsecase playerUsecase.PlayerUsecaseService
	}
)

func NewPlayerHttpHandlerService(cfg *config.Config, playerUsecase playerUsecase.PlayerUsecaseService) PlayerHttpHandlerService {
	return &playerHttpHandler{cfg: cfg, playerUsecase: playerUsecase}
}
