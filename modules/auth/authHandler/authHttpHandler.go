package authHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"
)

type (
	AuthHandlerService interface{}

	authHandler struct {
		config      *config.Config
		authUsecase authUsecase.AuthUsecaseService
	}
)

func NewAuthHandlerService(config *config.Config, authUsecase authUsecase.AuthUsecaseService) AuthHandlerService {
	return authHandler{config, authUsecase}
}
