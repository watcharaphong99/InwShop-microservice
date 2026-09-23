package grpccon

import (
	"context"
	"testing"

	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUnaryAuthorizationAcceptsApiKey(t *testing.T) {
	secret := "apisecret"
	g := &grpcAuth{secretKey: secret}
	token := jwtauth.NewApiKey(secret).SignToken()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("auth", token))

	res, err := g.unaryAuthorization(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unaryAuthorization: %v", err)
	}
	if res != "ok" {
		t.Fatalf("got %v, want ok", res)
	}
}

func TestUnaryAuthorizationRejectsAccessTokenSubject(t *testing.T) {
	secret := "apisecret"
	g := &grpcAuth{secretKey: secret}
	token := jwtauth.NewAccessToken(secret, 60, &jwtauth.Claims{PlayerId: "player:1", RoleCode: 1}).SignToken()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("auth", token))

	_, err := g.unaryAuthorization(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err == nil {
		t.Fatal("expected error for access-token subject")
	}
}

func TestUnaryAuthorizationRejectsMissingMetadata(t *testing.T) {
	g := &grpcAuth{secretKey: "apisecret"}
	_, err := g.unaryAuthorization(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err == nil {
		t.Fatal("expected error for missing metadata")
	}
}
