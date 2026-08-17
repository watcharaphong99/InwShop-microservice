package authRepository

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type (
	AuthRepositoryService interface{}

	authRepository struct {
		db *mongo.Client
	}
)

func NewRepository(db *mongo.Client) AuthRepositoryService {
	return &authRepository{db: db}
}

func (r *authRepository) authDbconn(pctx context.Context) *mongo.Database {
	return r.db.Database("auth_db")
}
