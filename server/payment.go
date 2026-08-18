package server

import (
	"github.com/watcharaphong99/InwzaShop/modules/payment/paymentHandler"
	"github.com/watcharaphong99/InwzaShop/modules/payment/paymentRepository"
	paymentUsecase "github.com/watcharaphong99/InwzaShop/modules/payment/paymentUseCase"
)

func (s *server) paymentService() {
	repo := paymentRepository.NewPaymentRepository(s.db)
	usecase := paymentUsecase.NewPaymentUsecase(repo)
	httpHander := paymentHandler.NewPaymentHttp(s.cfg, usecase)
	queue := paymentHandler.NewPaymentQueue(s.cfg, usecase)

	_ = httpHander
	_ = queue

	payment := s.app.Group("/v1/inventory")
	//help check
	payment.GET("", s.healthcheckService)

}
