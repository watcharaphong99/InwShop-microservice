package inventory

import (
	"errors"

	"github.com/watcharaphong99/InwzaShop/modules/item"
	"github.com/watcharaphong99/InwzaShop/modules/models"
)

type (
	UpdateInventoryReq struct {
		PlayerId string `json:"player_id" validate:"required,max=64"`
		ItemId   string `json:"item_id" validate:"required,max=64"`
		EventId  string `json:"event_id" validate:"required,max=128"`
	}

	ItemInInventory struct {
		InventoryId string `json:"inventory_id"`
		PlayerId    string `json:"player_id"`
		*item.ItemShowCase
	}

	InventorySearchReq struct {
		models.PaginateReq
	}

	RollbackPlayerInventoryReq struct {
		InventoryId string `json:"inventory_id"`
		PlayerId    string `json:"player_id"`
		ItemId      string `json:"item_id"`
		EventId     string `json:"event_id"`
	}
)

var (
	ErrItemNotFound   = errors.New("error: item not found")
	ErrEventCancelled = errors.New("error: event cancelled")
)
