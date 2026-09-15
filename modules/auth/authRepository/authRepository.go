package authRepository

import (
	"context"
	"errors"
	"log"
	"time"

	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"

	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
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

func (r *authRepository) CredentialSearch(pctx context.Context, grpcUrl string, req *playerPb.CredentialSearchReq) (*playerPb.PlayerProfile, error) {
	ctx, cancel := context.WithTimeout(pctx, 30*time.Second)
	defer cancel()

	conn, err := grpccon.NewGrpcClient(grpcUrl)
	if err != nil {
		log.Printf("Error: gRPC conection failed: %s", err.Error())
		return nil, errors.New("error: gRpc connection failed")
	}

	result, err := conn.Player().CredentialSearch(ctx, req)

	if err != nil {
		log.Printf("Error: CredentialSearch failed: %s", err.Error())
		return nil, errors.New(err.Error())
	}

	return result, nil
}
