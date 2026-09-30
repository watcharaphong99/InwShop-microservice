package itemHandler

import (
	"fmt"
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
		FindManyItems(c echo.Context) error
		EditItem(c echo.Context) error
		EnableOrDisableItem(c echo.Context) error
		DeleteItem(c echo.Context) error
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
	ctx := c.Request().Context()

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
	ctx := c.Request().Context()

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

func (h *itemHttpHandler) FindManyItems(c echo.Context) error {
	ctx := c.Request().Context()

	wrapper := request.ContextWrapper(c)

	req := new(item.ItemSearchReq)

	if err := wrapper.Bind(req); err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	res, err := h.itemUsecase.FindManyItems(ctx, h.cfg.Paginate.ItemNextPageBasedUrl, req)
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

func (h *itemHttpHandler) EditItem(c echo.Context) error {
	ctx := c.Request().Context()

	itemId := strings.TrimPrefix(c.Param("item_id"), "item:")

	wrapper := request.ContextWrapper(c)

	req := new(item.ItemUpdateReq)

	if err := wrapper.Bind(req); err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	res, err := h.itemUsecase.EditItem(ctx, itemId, req)

	if err != nil {
		switch err.Error() {
		case "error: id is invalid", "error: no fields to update":
			return response.ErrResponse(c, http.StatusBadRequest, err.Error())
		case "error: item not found":
			return response.ErrResponse(c, http.StatusNotFound, err.Error())
		case "error: this title is already exist":
			return response.ErrResponse(c, http.StatusConflict, err.Error())
		default:
			return response.ErrResponse(c, http.StatusInternalServerError, err.Error())
		}
	}

	return response.SuccessResponse(c, http.StatusOK, res)
}

func (h *itemHttpHandler) EnableOrDisableItem(c echo.Context) error {
	ctx := c.Request().Context()

	itemId := strings.TrimPrefix(c.Param("item_id"), "item:")

	wrapper := request.ContextWrapper(c)

	req := new(item.EnableOrDisableItemReq)

	if err := wrapper.Bind(req); err != nil {
		return response.ErrResponse(c, http.StatusBadRequest, err.Error())
	}

	res, err := h.itemUsecase.EnableOrDisableItem(ctx, itemId, *req.UsageStatus)
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

	return response.SuccessResponse(c, http.StatusOK, map[string]any{
		"message": fmt.Sprintf("item_id: %s usage_status changed to: %v", itemId, res),
	})
}

func (h *itemHttpHandler) DeleteItem(c echo.Context) error {
	ctx := c.Request().Context()

	itemId := strings.TrimPrefix(c.Param("item_id"), "item:")

	result, err := h.itemUsecase.DeleteItem(ctx, itemId)

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

	return response.SuccessResponse(c, http.StatusOK, &response.MsgResponse{
		Message: fmt.Sprintf("Deleted count: %d", result),
	})
}
