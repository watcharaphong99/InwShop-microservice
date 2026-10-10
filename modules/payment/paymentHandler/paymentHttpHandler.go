package paymentHandler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
	paymentUsecase "github.com/watcharaphong99/InwzaShop/modules/payment/paymentUseCase"
	"github.com/watcharaphong99/InwzaShop/pkg/request"
	"github.com/watcharaphong99/InwzaShop/pkg/response"
)

type (
	PaymentHttpHandlerService interface {
		PlaceOrder(c echo.Context) error
		SellItem(c echo.Context) error
	}

	paymentHttpHandler struct {
		cfg            *config.Config
		paymentUsecase paymentUsecase.PaymentUsecaseService
	}
)

func NewPaymentHttpHandler(cfg *config.Config, paymentUsecase paymentUsecase.PaymentUsecaseService) PaymentHttpHandlerService {
	return &paymentHttpHandler{
		cfg:            cfg,
		paymentUsecase: paymentUsecase,
	}
}

// PlaceOrder HTTP entry สำหรับซื้อ (เรียก ExecutePurchase)
func (h *paymentHttpHandler) PlaceOrder(c echo.Context) error {
	playerId, ok := c.Get("player_id").(string)
	if !ok || playerId == "" {
		return response.ErrResponse(c, http.StatusUnauthorized, "error: unauthorized")
	}

	req := &payment.ItemServiceReq{
		Items: make([]*payment.ItemServiceReqDatum, 0),
	}
	if err := request.ContextWrapper(c).Bind(req); err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	res, err := h.paymentUsecase.ExecutePurchase(c.Request().Context(), h.cfg, playerId, req)
	if err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	return response.SuccessResponse(c, http.StatusOK, res)
}

func (h *paymentHttpHandler) SellItem(c echo.Context) error {
	playerId, ok := c.Get("player_id").(string)
	if !ok || playerId == "" {
		return response.ErrResponse(c, http.StatusUnauthorized, "error: unauthorized")
	}

	req := &payment.ItemServiceReq{
		Items: make([]*payment.ItemServiceReqDatum, 0),
	}
	if err := request.ContextWrapper(c).Bind(req); err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	res, err := h.paymentUsecase.SellItem(c.Request().Context(), h.cfg, playerId, req)
	if err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	return response.SuccessResponse(c, http.StatusOK, res)
}
