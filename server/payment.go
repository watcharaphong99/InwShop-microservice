package server

import (
	"github.com/watcharaphong99/InwzaShop/modules/payment/paymentHandler"
	"github.com/watcharaphong99/InwzaShop/modules/payment/paymentRepository"
	paymentUsecase "github.com/watcharaphong99/InwzaShop/modules/payment/paymentUseCase"
)

func (s *server) paymentService() {
	repo := paymentRepository.NewPaymentRepository(s.db)
	usecase := paymentUsecase.NewPaymentUsecase(repo)
	httpHandler := paymentHandler.NewPaymentHttpHandler(s.cfg, usecase)
	queue := paymentHandler.NewPaymentQueue(s.cfg, usecase)

	_ = queue

	payment := s.app.Group("/payment_v1")
	//help check
	payment.GET("", s.healthcheckService)

	payment.POST("/payment/buy", httpHandler.BuyItem, s.middleware.JwtAuthorization)
	payment.POST("/payment/sell", httpHandler.SellItem, s.middleware.JwtAuthorization)

}
