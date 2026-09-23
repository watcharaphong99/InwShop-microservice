package middlewareRepository

import (
	"context"
	"errors"
	"log"
	"time"

	authpb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	"github.com/watcharaphong99/InwzaShop/pkg/grpccon"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
)

type (
	MiddlewareRepositoryService interface {
		AccessTokenSearch(pctx context.Context, grpc, accessToken string) error
		RolesCount(pctx context.Context, grpcUrl string) (int64, error)
	}

	middlewarerepository struct {
		cache     *rediscon.Client
		accessTTL time.Duration
	}
)

func NewMiddlewarerepository(cache *rediscon.Client, accessTTL time.Duration) MiddlewareRepositoryService {
	return &middlewarerepository{
		cache:     cache,
		accessTTL: accessTTL,
	}
}

func (r *middlewarerepository) AccessTokenSearch(pctx context.Context, grpcUrl, accessToken string) error {
	ctx, cancle := context.WithTimeout(pctx, 30*time.Second)
	defer cancle()

	if r.cache.HasAccessToken(ctx, accessToken) {
		return nil
	}

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
		log.Printf("Error: AccessTokenSearch failed: %s", err.Error())
		return errors.New("error: access token is invalid")
	}

	if result == nil {
		log.Printf("Error: access token is invalid")
		return errors.New("error: access token is invalid")
	}

	if !result.IsValid {
		log.Printf("Error: access token is invalid")
		return errors.New("error: access token is invalid")
	}

	r.cache.SetAccessToken(ctx, accessToken, r.accessTTL)

	return nil
}

func (r *middlewarerepository) RolesCount(pctx context.Context, grpcUrl string) (int64, error) {
	ctx, cancle := context.WithTimeout(pctx, 30*time.Second)
	defer cancle()

	if count, ok := r.cache.GetRolesCount(ctx); ok {
		return count, nil
	}

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

	r.cache.SetRolesCount(ctx, result.Count)

	return result.Count, nil
}
