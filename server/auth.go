package server

import (
	"github.com/watcharaphong99/InwzaShop/modules/auth/authHandler"
	"github.com/watcharaphong99/InwzaShop/modules/auth/authRepository"
	authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"
)

func (s *server) authService() {

	repo := authRepository.NewRepository(s.db)
	usecase := authUsecase.NewAuthUseCase(repo)
	httpHandler := authHandler.NewAuthHandlerService(s.cfg, usecase)
	grpcHandler := authHandler.NewAuthGrpcHandler(usecase)

	_ = httpHandler
	_ = grpcHandler

	auth := s.app.Group("/auth_v1")

	//help check
	auth.GET("", s.healthcheckService)
}
