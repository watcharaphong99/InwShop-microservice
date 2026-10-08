package payment

import "time"

type (
	ItemServiceReq struct {
		Items []*ItemServiceReqDatum `json:"items" validate:"required"`
	}

	ItemServiceReqDatum struct {
		ItemId string  `json:"item_id" validate:"required,max=64"`
		Price  float64 `json:"price"`
	}

	PaymentTransferReq struct {
		PlayerId string  `json:"player_id"`
		ItemId   string  `json:"item_id"`
		Amout    float64 `json:"amount"`
	}

	PaymentTransferRes struct {
		EventId       string  `json:"event_id"`
		InventoryId   string  `json:"inventory_id"`
		TransactionId string  `json:"transaction_id"`
		PlayerId      string  `json:"player_id"`
		ItemId        string  `json:"item_id"`
		Amount        float64 `json:"amount"`
		Error         string  `json:"error"`
	}

	Saga struct {
		ID        string     `bson:"_id"`
		PlayerID  string     `bson:"player_id"`
		Action    string     `bson:"action"`
		Status    string     `bson:"status"`
		Steps     []SagaStep `bson:"steps"`
		UpdatedAt time.Time  `bson:"updated_at"`
	}

	SagaStep struct {
		EventID       string  `bson:"event_id"`
		Kind          string  `bson:"kind"`
		ItemID        string  `bson:"item_id"`
		Amount        float64 `bson:"amount"`
		TransactionID string  `bson:"transaction_id,omitempty"`
		InventoryID   string  `bson:"inventory_id,omitempty"`
		Status        string  `bson:"status"`
	}
)
