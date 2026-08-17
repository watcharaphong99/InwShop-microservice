package middlewareHandler

import (
	"github.com/watcharaphong99/InwzaShop/config"
	middlewareusecase "github.com/watcharaphong99/InwzaShop/modules/middlerware/middlewareUsecase"
)

type (
	MiddlewareHandlerService interface{}

	middlewareHandler struct {
		cfg               *config.Config
		middlewareUsecase middlewareusecase.MiddlewareUsecaseService
	}
)

func NewMiddlewareHandler(cfg *config.Config, middlewareUsecase middlewareusecase.MiddlewareUsecaseService) MiddlewareHandlerService {
	return &middlewareHandler{
		cfg:               cfg,
		middlewareUsecase: middlewareUsecase}
}
