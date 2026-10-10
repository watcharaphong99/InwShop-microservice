package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/watcharaphong99/InwzaShop/modules/auth"
	"github.com/watcharaphong99/InwzaShop/modules/auth/authRepository"
	authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newAuthUsecaseForTest(t *testing.T, repo authRepository.AuthRepositoryService) authUsecase.AuthUsecaseService {
	t.Helper()
	cfg := NewTestConfig()
	cache := rediscon.NewClient("")
	return authUsecase.NewAuthUseCase(repo, cache, cfg.Jwt.AccessDuration)
}

func testLoginProfile(nowRFC3339 string) *playerPb.PlayerProfile {
	return &playerPb.PlayerProfile{
		Id:        "507f1f77bcf86cd799439011",
		Email:     "player@example.com",
		Username:  "player1",
		RoleCode:  1,
		CreatedAt: nowRFC3339,
		UpdatedAt: nowRFC3339,
	}
}

// TestLogin_Success — happy path: token ที่ client ได้ตรงกับที่ insert + JWT claims ถูก
func TestLogin_Success(t *testing.T) {
	ctx := context.Background()
	cfg := NewTestConfig()

	repo := authRepository.NewAuthRepositoryMock().(*authRepository.AuthRepositoryMock)
	uc := newAuthUsecaseForTest(t, repo)

	credentialID := primitive.NewObjectID()
	nowRFC3339 := time.Now().UTC().Format(time.RFC3339Nano)
	profile := testLoginProfile(nowRFC3339)

	repo.On("CredentialSearch", ctx, cfg.Grpc.PlayerUrl, mock.MatchedBy(func(req *playerPb.CredentialSearchReq) bool {
		return req.Email == "player@example.com" && req.Password == "secret"
	})).Return(profile, nil)

	var inserted *auth.Credential
	repo.On("InsertOnePlayerCredential", ctx, mock.AnythingOfType("*auth.Credential")).
		Run(func(args mock.Arguments) {
			c := args.Get(1).(*auth.Credential)
			inserted = &auth.Credential{
				Id:           credentialID,
				PlayerId:     c.PlayerId,
				RoleCode:     c.RoleCode,
				AccessToken:  c.AccessToken,
				RefreshToken: c.RefreshToken,
				CreatedAt:    c.CreatedAt,
				UpdatedAt:    c.UpdatedAt,
			}
		}).
		Return(credentialID, nil)

	repo.On("FindOnePlayerCredential", ctx, credentialID.Hex()).
		Return(func(context.Context, string) (*auth.Credential, error) {
			return inserted, nil
		})

	res, err := uc.Login(ctx, cfg, &auth.PlayerLoginReq{
		Email:    "player@example.com",
		Password: "secret",
	})

	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, inserted)
	assert.Equal(t, profile.Email, res.PlayerProfile.Email)
	assert.Equal(t, inserted.AccessToken, res.Credential.AccessToken)
	assert.Equal(t, inserted.RefreshToken, res.Credential.RefreshToken)
	assert.NotEmpty(t, res.Credential.AccessToken)

	accessClaims, err := jwtauth.ParseToken(cfg.Jwt.AccessSecretKey, res.Credential.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "player:507f1f77bcf86cd799439011", accessClaims.PlayerId)
	assert.Equal(t, 1, accessClaims.RoleCode)
	assert.Equal(t, "access-token", accessClaims.Subject)

	refreshClaims, err := jwtauth.ParseToken(cfg.Jwt.RefreshSecretKey, res.Credential.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "refresh-token", refreshClaims.Subject)

	repo.AssertExpectations(t)
}

// TestLogin_CredentialSearchFails — login ไม่ผ่าน gRPC/player ไม่ insert
func TestLogin_CredentialSearchFails(t *testing.T) {
	ctx := context.Background()
	cfg := NewTestConfig()

	repo := authRepository.NewAuthRepositoryMock().(*authRepository.AuthRepositoryMock)
	uc := newAuthUsecaseForTest(t, repo)

	searchErr := errors.New("error: invalid email or password")
	repo.On("CredentialSearch", ctx, cfg.Grpc.PlayerUrl, mock.Anything).
		Return((*playerPb.PlayerProfile)(nil), searchErr)

	res, err := uc.Login(ctx, cfg, &auth.PlayerLoginReq{
		Email:    "wrong@example.com",
		Password: "wrong",
	})

	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, searchErr)
	repo.AssertNotCalled(t, "InsertOnePlayerCredential")
	repo.AssertNotCalled(t, "FindOnePlayerCredential")
	repo.AssertExpectations(t)
}

// TestLogin_InsertFails — หลัง sign JWT แล้ว insert พัง ไม่ควร find credential
func TestLogin_InsertFails(t *testing.T) {
	ctx := context.Background()
	cfg := NewTestConfig()

	repo := authRepository.NewAuthRepositoryMock().(*authRepository.AuthRepositoryMock)
	uc := newAuthUsecaseForTest(t, repo)

	repo.On("CredentialSearch", ctx, cfg.Grpc.PlayerUrl, mock.MatchedBy(func(req *playerPb.CredentialSearchReq) bool {
		return req.Email == "player@example.com" && req.Password == "secret"
	})).Return(testLoginProfile(time.Now().UTC().Format(time.RFC3339Nano)), nil)

	insertErr := errors.New("error: insert one player credential failed")
	repo.On("InsertOnePlayerCredential", ctx, mock.AnythingOfType("*auth.Credential")).
		Return(primitive.NilObjectID, insertErr)

	res, err := uc.Login(ctx, cfg, &auth.PlayerLoginReq{
		Email:    "player@example.com",
		Password: "secret",
	})

	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, insertErr)
	repo.AssertNotCalled(t, "FindOnePlayerCredential")
	repo.AssertExpectations(t)
}
