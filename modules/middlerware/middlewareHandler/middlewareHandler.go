package middlewareHandler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/watcharaphong99/InwzaShop/config"
	middlewareusecase "github.com/watcharaphong99/InwzaShop/modules/middlerware/middlewareUsecase"
	"github.com/watcharaphong99/InwzaShop/pkg/response"
)

type (
	MiddlewareHandlerService interface {
		JwtAuthorization(next echo.HandlerFunc) echo.HandlerFunc
		RbacAuthorization(next echo.HandlerFunc, expected []int) echo.HandlerFunc
		PlayerIdParamValidation(next echo.HandlerFunc) echo.HandlerFunc
	}

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

func (h *middlewareHandler) JwtAuthorization(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		accessToken := strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer ")

		newCtx, err := h.middlewareUsecase.JwtAuthorization(c, h.cfg, accessToken)
		if err != nil {
			return response.ErrResponse(c, http.StatusUnauthorized, err.Error())
		}

		return next(newCtx)
	}
}

func (h *middlewareHandler) RbacAuthorization(next echo.HandlerFunc, expected []int) echo.HandlerFunc {
	return func(c echo.Context) error {

		newCtx, err := h.middlewareUsecase.RbacAuthorization(c, h.cfg, expected)
		if err != nil {
			return response.ErrResponse(c, http.StatusUnauthorized, err.Error())
		}

		return next(newCtx)
	}
}

func (h *middlewareHandler) PlayerIdParamValidation(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		nexCtx, err := h.middlewareUsecase.PlayerIdParamValidation(c)
		if err != nil {
			return response.ErrResponse(c, http.StatusUnauthorized, err.Error())
		}

		return next(nexCtx)
	}
}
