package authHandler

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/auth"
	authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"
	"github.com/watcharaphong99/InwzaShop/pkg/request"
	"github.com/watcharaphong99/InwzaShop/pkg/response"
)

type (
	AuthHttpHandlerService interface {
		Login(c echo.Context) error
	}

	authHttpHandler struct {
		config      *config.Config
		authUsecase authUsecase.AuthUsecaseService
	}
)

func NewAuthHandlerService(config *config.Config, authUsecase authUsecase.AuthUsecaseService) AuthHttpHandlerService {
	return &authHttpHandler{config, authUsecase}
}

func (h *authHttpHandler) Login(c echo.Context) error {
	ctx := context.Background()

	wrapper := request.ContextWrapper(c)

	req := new(auth.PlayerLoginReq)

	if err := wrapper.Bind(req); err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	res, err := h.authUsecase.Login(ctx, h.config, req)
	if err != nil {
		return response.ErrResponse(c, http.StatusUnauthorized, err.Error())
	}

	return response.SuccessResponse(c, http.StatusOK, res)
}
