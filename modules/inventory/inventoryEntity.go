package inventory

import "go.mongodb.org/mongo-driver/bson/primitive"

type (
	Inventory struct {
		Id             primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
		PlayerId       string             `json:"player_id" bson:"player_id"`
		ItemId         string             `json:"item_id" bson:"item_id"`
		EventId        string             `json:"event_id,omitempty" bson:"event_id,omitempty"`
		RemovedEventId string             `json:"removed_event_id,omitempty" bson:"removed_event_id,omitempty"`
	}
)
