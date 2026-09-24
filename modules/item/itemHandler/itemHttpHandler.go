package itemHandler

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/item"
	itemUsecase "github.com/watcharaphong99/InwzaShop/modules/item/itemUseCase"
	"github.com/watcharaphong99/InwzaShop/pkg/request"
	"github.com/watcharaphong99/InwzaShop/pkg/response"
)

type (
	ItemHttpHandlerService interface {
		CreateItem(c echo.Context) error
		FindOneItem(c echo.Context) error
	}

	itemHttpHandler struct {
		cfg         *config.Config
		itemUsecase itemUsecase.ItemUsecaseService
	}
)

func NewItemHttpHandler(cfg *config.Config, itemUsecase itemUsecase.ItemUsecaseService) ItemHttpHandlerService {
	return &itemHttpHandler{cfg: cfg, itemUsecase: itemUsecase}
}

func (h *itemHttpHandler) CreateItem(c echo.Context) error {
	ctx := context.Background()

	wrappers := request.ContextWrapper(c)

	req := new(item.CreateItemReq)

	if err := wrappers.Bind(req); err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	res, err := h.itemUsecase.CreateItem(ctx, req)
	if err != nil {
		switch err.Error() {
		case "error: this title is already exist":
			return response.ErrResponse(c, http.StatusConflict, err.Error())
		default:
			return response.ErrResponse(c, http.StatusInternalServerError, err.Error())
		}
	}

	return response.SuccessResponse(c, http.StatusCreated, res)
}

func (h *itemHttpHandler) FindOneItem(c echo.Context) error {
	ctx := context.Background()

	itemId := strings.TrimPrefix(c.Param("item_id"), "item:")

	res, err := h.itemUsecase.FindOneItem(ctx, itemId)
	if err != nil {
		switch err.Error() {
		case "error: id is invalid":
			return response.ErrResponse(c, http.StatusBadRequest, err.Error())
		case "error: item not found":
			return response.ErrResponse(c, http.StatusNotFound, err.Error())
		default:
			return response.ErrResponse(c, http.StatusInternalServerError, err.Error())
		}
	}

	return response.SuccessResponse(c, http.StatusOK, res)
}
