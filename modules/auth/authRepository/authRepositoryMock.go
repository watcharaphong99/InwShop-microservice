package authRepository

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/watcharaphong99/InwzaShop/modules/auth"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthRepositoryMock struct {
	mock.Mock
}

func NewAuthRepositoryMock() AuthRepositoryService {
	return &AuthRepositoryMock{}
}

func (m *AuthRepositoryMock) InsertOnePlayerCredential(pctx context.Context, req *auth.Credential) (primitive.ObjectID, error) {
	args := m.Called(pctx, req)
	return args.Get(0).(primitive.ObjectID), args.Error(1)
}

func (m *AuthRepositoryMock) CredentialSearch(pctx context.Context, grpcUrl string, req *playerPb.CredentialSearchReq) (*playerPb.PlayerProfile, error) {
	args := m.Called(pctx, grpcUrl, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*playerPb.PlayerProfile), args.Error(1)
}

func (m *AuthRepositoryMock) FindOnePlayerCredential(pctx context.Context, credentialId string) (*auth.Credential, error) {
	args := m.Called(pctx, credentialId)
	if fn, ok := args.Get(0).(func(context.Context, string) (*auth.Credential, error)); ok {
		return fn(pctx, credentialId)
	}
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Credential), args.Error(1)
}

func (m *AuthRepositoryMock) FindOnePlayerProfileToRefresh(pctx context.Context, grpcUrl string, req *playerPb.FindOnePlayerProfileToRefreshReq) (*playerPb.PlayerProfile, error) {
	return nil, nil
}

func (m *AuthRepositoryMock) UpdateOnePlayerCredential(pctx context.Context, credentialId string, req *auth.UpdateRefreshTokenReq) error {
	return nil
}

func (m *AuthRepositoryMock) DeleteOnePlayerCredential(pctx context.Context, credentialId string) (int64, error) {
	return 0, nil
}

func (m *AuthRepositoryMock) FindOneAccessToken(pctx context.Context, AccessToken string) (*auth.Credential, error) {
	return nil, nil
}

func (m *AuthRepositoryMock) RolesCount(pctx context.Context) (int64, error) {
	return 0, nil
}
