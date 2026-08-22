package inventoryHandler

import (
	"context"

	authpb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	inventoryPb "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryPb"
	inventoryusecase "github.com/watcharaphong99/InwzaShop/modules/inventory/inventoryUseCase"
)

type (
	inventoryGrpcHandler struct {
		inventoryusecase inventoryusecase.InventoryUsecaseService
		inventoryPb.UnimplementedInventoryGrpcServiceServer
	}
)

// AccessTokenSearch implements [authpb.AuthGrpcServiceServer].
func (g *inventoryGrpcHandler) AccessTokenSearch(context.Context, *authpb.AccessTokenSearchReq) (*authpb.AccessTokenSearchRes, error) {
	panic("unimplemented")
}

// RolesCount implements [authpb.AuthGrpcServiceServer].
func (g *inventoryGrpcHandler) RolesCount(context.Context, *authpb.RolesCountReq) (*authpb.RolesCountRes, error) {
	panic("unimplemented")
}

// mustEmbedUnimplementedAuthGrpcServiceServer implements [authpb.AuthGrpcServiceServer].
func (g *inventoryGrpcHandler) mustEmbedUnimplementedAuthGrpcServiceServer() {
	panic("unimplemented")
}

func NewInventoryGrpcHandler(inventoryusecase inventoryusecase.InventoryUsecaseService) *inventoryGrpcHandler {
	return &inventoryGrpcHandler{
		inventoryusecase: inventoryusecase,
	}
}

func (g *inventoryGrpcHandler) IsAvailableToSell(ctx context.Context, req *inventoryPb.IsAvailableToSellReq) (*inventoryPb.IsAvailableToSellRes, error) {
	return nil, nil
}
