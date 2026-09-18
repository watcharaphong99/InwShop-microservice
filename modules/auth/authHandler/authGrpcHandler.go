package authHandler

import (
	authPb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"
)

type (
	authGrpcHandler struct {
		authPb.UnimplementedAuthGrpcServiceServer
		authUsecase authUsecase.AuthUsecaseService
	}
)

func NewAuthGrpcHandler(authUsecase authUsecase.AuthUsecaseService) *authGrpcHandler {
	return &authGrpcHandler{
		authUsecase: authUsecase,
	}
}
