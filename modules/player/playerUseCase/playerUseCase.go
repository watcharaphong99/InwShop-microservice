package playerUsecase

import "github.com/watcharaphong99/InwzaShop/modules/player/playerRepository"

type (
	PlayerUsecaseService interface{}

	playerUsecase struct {
		playerRepository playerRepository.PlayerRepositoryService
	}
)

func NewPlayerUsecaseService(playerRepository playerRepository.PlayerRepositoryService) PlayerUsecaseService {
	return playerUsecase{playerRepository: playerRepository}
}
