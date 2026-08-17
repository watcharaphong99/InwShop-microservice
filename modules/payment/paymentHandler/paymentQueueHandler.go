package paymentHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	paymentUsecase "github.com/watcharaphong99/InwzaShop/modules/payment/paymentUseCase"
)

type (
	PaymentQueueHandlerService interface{}

	paymentQueueHandler struct {
		config         *config.Config
		paymentUsecase paymentUsecase.PaymentUsecaseService
	}
)

func NewPaymentQueue(config *config.Config, paymentUsecase paymentUsecase.PaymentUsecaseService) PaymentQueueHandlerService {
	return &paymentQueueHandler{
		config:         config,
		paymentUsecase: paymentUsecase}
}
