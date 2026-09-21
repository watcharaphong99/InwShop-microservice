package middlewareRepository

import (
	"context"
	"errors"
	"log"
	"time"

	authpb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
)

type (
	MiddlewareRepositoryService interface {
		AccessTokenSearch(pctx context.Context, grpc, accessToken string) error
		RolesCount(pctx context.Context, grpcUrl string) (int64, error)
	}

	middlewarerepository struct{}
)

func NewMiddlewarerepository() MiddlewareRepositoryService {
	return &middlewarerepository{}
}

func (r *middlewarerepository) AccessTokenSearch(pctx context.Context, grpcUrl, accessToken string) error {
	ctx, cancle := context.WithTimeout(pctx, 30*time.Second)
	defer cancle()

	conn, err := grpccon.NewGrpcClient(grpcUrl)
	if err != nil {
		log.Printf("Error: gRPC connection failed: %s", err.Error())
		return errors.New("error: gRpc connection failed")
	}
	defer conn.Close()

	jwtauth.SetApiKeyInContext(&ctx)
	result, err := conn.Auth().AccessTokenSearch(ctx, &authpb.AccessTokenSearchReq{
		AccessToken: accessToken,
	})
	if err != nil {
		log.Printf("Error: CredentialSearch failed: %s", err.Error())
		return errors.New("error: email or password is incorrect")
	}

	if result == nil {
		log.Printf("Error: access token is invalid")
		return errors.New("error: access token is invalid")
	}

	if !result.IsValid {
		log.Printf("Error: access token is invalid")
		return errors.New("error: access token is invalid")
	}

	return nil
}

func (r *middlewarerepository) RolesCount(pctx context.Context, grpcUrl string) (int64, error) {
	ctx, cancle := context.WithTimeout(pctx, 30*time.Second)
	defer cancle()

	conn, err := grpccon.NewGrpcClient(grpcUrl)
	if err != nil {
		log.Printf("Error: gRPC connection failed: %s", err.Error())
		return 0, errors.New("error: gRpc connection failed")
	}
	defer conn.Close()

	jwtauth.SetApiKeyInContext(&ctx)
	result, err := conn.Auth().RolesCount(ctx, &authpb.RolesCountReq{})
	if err != nil {
		log.Printf("Error: RolesCount failed: %s", err.Error())
		return 0, errors.New("error: roles count failed")
	}

	if result == nil {
		log.Printf("Error: RolesCount result is nil")
		return 0, errors.New("error: roles count failed")
	}

	return result.Count, nil
}
