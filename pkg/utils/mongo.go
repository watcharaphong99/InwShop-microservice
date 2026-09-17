package utils

import (
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ConvertToObjectId(id string) primitive.ObjectID {
	objectId, _ := primitive.ObjectIDFromHex(id)
	return objectId
}

func ParseObjectId(id string) (primitive.ObjectID, error) {
	if !primitive.IsValidObjectID(id) {
		return primitive.NilObjectID, errors.New("error: id is invalid")
	}
	return primitive.ObjectIDFromHex(id)
}
