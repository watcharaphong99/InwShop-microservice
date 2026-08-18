package playerHandler

import playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"

type (
	playerGrpcHandlerService struct {
		playerUsecase playerUsecase.PlayerUsecaseService
	}
)

func NewPlayerGrpcHandlerService(playerUsecase playerUsecase.PlayerUsecaseService) *playerGrpcHandlerService {
	return &playerGrpcHandlerService{playerUsecase: playerUsecase}
}
