package authHandler

import authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"

type (
	authGrpcHandler struct {
		authUsecase authUsecase.AuthUsecaseService
	}
)

func NewAuthGrpcHandler(authUsecase authUsecase.AuthUsecaseService) *authGrpcHandler {
	return &authGrpcHandler{authUsecase}
}
