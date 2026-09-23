package grpccon

import (
	"context"
	"errors"
	"log"
	"net"

	authPb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"

	inventoryPb "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryPb"
	itemPb "github.com/watcharaphong99/InwzaShop/modules/item/itemPb"

	"github.com/watcharaphong99/InwzaShop/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type (
	GrpcClientFactoryHandler interface {
		Auth() authPb.AuthGrpcServiceClient
		Player() playerPb.PlayerGrpcServiceClient
		Inventory() inventoryPb.InventoryGrpcServiceClient
		Item() itemPb.ItemGrpcServiceClient
		Close() error
	}

	grpcClientFactory struct {
		client *grpc.ClientConn
	}

	grpcAuth struct {
		secretKey string
	}
)

func (g *grpcAuth) unaryAuthorization(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Printf("Error: metadata not found")
		return nil, errors.New("error: metadata not found")
	}

	authHeader, ok := md["auth"]
	if !ok || len(authHeader) == 0 {
		log.Printf("Error: auth metadata not found")
		return nil, errors.New("error: metadata not found")
	}

	claims, err := jwtauth.ParseToken(g.secretKey, authHeader[0])
	if err != nil {
		log.Printf("Error: parse token failed: %s", err.Error())
		return nil, errors.New("error: token is invalid")
	}

	if claims.Subject != "api-key" {
		log.Printf("Error: token subject is invalid")
		return nil, errors.New("error: token is invalid")
	}

	return handler(ctx, req)
}

func (g *grpcClientFactory) Auth() authPb.AuthGrpcServiceClient {
	return authPb.NewAuthGrpcServiceClient(g.client)
}

func (g *grpcClientFactory) Player() playerPb.PlayerGrpcServiceClient {
	return playerPb.NewPlayerGrpcServiceClient(g.client)
}

func (g *grpcClientFactory) Inventory() inventoryPb.InventoryGrpcServiceClient {
	return inventoryPb.NewInventoryGrpcServiceClient(g.client)
}

func (g *grpcClientFactory) Item() itemPb.ItemGrpcServiceClient {
	return itemPb.NewItemGrpcServiceClient(g.client)
}

func (g *grpcClientFactory) Close() error {
	if g.client == nil {
		return nil
	}
	return g.client.Close()
}

func NewGrpcClient(host string) (GrpcClientFactoryHandler, error) {
	opts := make([]grpc.DialOption, 0)

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	clientConn, err := grpc.NewClient(host, opts...)
	if err != nil {
		log.Printf("Error: Grpc client connection failed: %s", err.Error())
		return nil, errors.New("error: grpc client connection failed")
	}

	return &grpcClientFactory{
		client: clientConn,
	}, nil
}

func NewGrpcServer(cfg *config.Jwt, host string) (*grpc.Server, net.Listener) {
	opts := make([]grpc.ServerOption, 0)

	grpcAuth := &grpcAuth{
		secretKey: cfg.ApiSceretKey,
	}

	opts = append(opts, grpc.UnaryInterceptor(grpcAuth.unaryAuthorization))

	grpcServer := grpc.NewServer(opts...)

	list, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("Error: failed to listen: %v", err)
	}

	return grpcServer, list
}
