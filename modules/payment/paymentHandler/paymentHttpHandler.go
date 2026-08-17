package paymentHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	paymentUsecase "github.com/watcharaphong99/InwzaShop/modules/payment/paymentUseCase"
)

type (
	PaymentHttpHandlerService interface{}

	paymentHttpHandler struct {
		config         *config.Config
		paymentUsecase paymentUsecase.PaymentUsecaseService
	}
)

func NewPaymentHttp(config *config.Config, paymentUsecase paymentUsecase.PaymentUsecaseService) PaymentHttpHandlerService {
	return &paymentHttpHandler{
		config:         config,
		paymentUsecase: paymentUsecase}
}
