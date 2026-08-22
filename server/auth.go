package server

import (
	"log"

	"github.com/watcharaphong99/InwzaShop/modules/auth/authHandler"
	authPb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	"github.com/watcharaphong99/InwzaShop/modules/auth/authRepository"
	authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
)

func (s *server) authService() {

	repo := authRepository.NewRepository(s.db)
	usecase := authUsecase.NewAuthUseCase(repo)
	httpHandler := authHandler.NewAuthHandlerService(s.cfg, usecase)
	grpcHandler := authHandler.NewAuthGrpcHandler(usecase)

	//gRpc
	go func() {
		grpcServer, lis := grpccon.NewGrpcServer(&s.cfg.Jwt, s.cfg.Grpc.AuthUrl)

		authPb.RegisterAuthGrpcServiceServer(grpcServer, grpcHandler)
		log.Printf("Auth gRPC server listening on %s", s.cfg.Grpc.AuthUrl)
		grpcServer.Serve(lis)
	}()

	_ = httpHandler
	_ = grpcHandler

	auth := s.app.Group("/auth_v1")

	//help check
	auth.GET("", s.healthcheckService)
}
