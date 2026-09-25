package authUsecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/auth"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var errNotImplemented = errors.New("fake: not implemented")

type fakeAuthRepo struct {
	insertOneCredentialFn func(ctx context.Context, req *auth.Credential) (primitive.ObjectID, error)
	credentialSearchFn    func(ctx context.Context, grpcUrl string, req *playerPb.CredentialSearchReq) (*playerPb.PlayerProfile, error)
	findOneCredentialFn   func(ctx context.Context, credentialId string) (*auth.Credential, error)
	findProfileRefreshFn  func(ctx context.Context, grpcUrl string, req *playerPb.FindOnePlayerProfileToRefreshReq) (*playerPb.PlayerProfile, error)
	updateCredentialFn    func(ctx context.Context, credentialId string, req *auth.UpdateRefreshTokenReq) error
	deleteCredentialFn    func(ctx context.Context, credentialId string) (int64, error)
	findOneAccessTokenFn  func(ctx context.Context, accessToken string) (*auth.Credential, error)
	rolesCountFn          func(ctx context.Context) (int64, error)
}

func (f *fakeAuthRepo) InsertOnePlayerCredential(ctx context.Context, req *auth.Credential) (primitive.ObjectID, error) {
	if f.insertOneCredentialFn == nil {
		return primitive.NilObjectID, errNotImplemented
	}
	return f.insertOneCredentialFn(ctx, req)
}

func (f *fakeAuthRepo) CredentialSearch(ctx context.Context, grpcUrl string, req *playerPb.CredentialSearchReq) (*playerPb.PlayerProfile, error) {
	if f.credentialSearchFn == nil {
		return nil, errNotImplemented
	}
	return f.credentialSearchFn(ctx, grpcUrl, req)
}

func (f *fakeAuthRepo) FindOnePlayerCredential(ctx context.Context, credentialId string) (*auth.Credential, error) {
	if f.findOneCredentialFn == nil {
		return nil, errNotImplemented
	}
	return f.findOneCredentialFn(ctx, credentialId)
}

func (f *fakeAuthRepo) FindOnePlayerProfileToRefresh(ctx context.Context, grpcUrl string, req *playerPb.FindOnePlayerProfileToRefreshReq) (*playerPb.PlayerProfile, error) {
	if f.findProfileRefreshFn == nil {
		return nil, errNotImplemented
	}
	return f.findProfileRefreshFn(ctx, grpcUrl, req)
}

func (f *fakeAuthRepo) UpdateOnePlayerCredential(ctx context.Context, credentialId string, req *auth.UpdateRefreshTokenReq) error {
	if f.updateCredentialFn == nil {
		return errNotImplemented
	}
	return f.updateCredentialFn(ctx, credentialId, req)
}

func (f *fakeAuthRepo) DeleteOnePlayerCredential(ctx context.Context, credentialId string) (int64, error) {
	if f.deleteCredentialFn == nil {
		return 0, errNotImplemented
	}
	return f.deleteCredentialFn(ctx, credentialId)
}

func (f *fakeAuthRepo) FindOneAccessToken(ctx context.Context, accessToken string) (*auth.Credential, error) {
	if f.findOneAccessTokenFn == nil {
		return nil, errNotImplemented
	}
	return f.findOneAccessTokenFn(ctx, accessToken)
}

func (f *fakeAuthRepo) RolesCount(ctx context.Context) (int64, error) {
	if f.rolesCountFn == nil {
		return 0, errNotImplemented
	}
	return f.rolesCountFn(ctx)
}

func testConfig() *config.Config {
	return &config.Config{
		Jwt: config.Jwt{
			AccessSecretKey:  "access-secret",
			RefreshSecretKey: "refresh-secret",
			AccessDuration:   60,
			RefreshDuration:  3600,
		},
		Grpc: config.Grpc{PlayerUrl: "player:1234"},
	}
}

func newTestUsecase(repo *fakeAuthRepo) AuthUsecaseService {
	return NewAuthUseCase(repo, &rediscon.Client{}, 60)
}

func testProfile() *playerPb.PlayerProfile {
	return &playerPb.PlayerProfile{
		Id:        "6a927a8535887829348adce9",
		Email:     "test@inwza.com",
		Username:  "tester",
		RoleCode:  1,
		CreatedAt: "2026-01-01T10:00:00+07:00",
		UpdatedAt: "2026-01-01T10:00:00+07:00",
	}
}

func assertErr(t *testing.T, err error, wantErr string) {
	t.Helper()
	if wantErr == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("want error containing %q, got %v", wantErr, err)
	}
}

func TestLogin(t *testing.T) {
	cfg := testConfig()
	credentialId := primitive.NewObjectID()

	// Stores whatever Login inserts so FindOne returns the same credential.
	newRepo := func() *fakeAuthRepo {
		var stored *auth.Credential
		return &fakeAuthRepo{
			credentialSearchFn: func(_ context.Context, url string, req *playerPb.CredentialSearchReq) (*playerPb.PlayerProfile, error) {
				if url != cfg.Grpc.PlayerUrl {
					t.Errorf("grpc url = %q", url)
				}
				return testProfile(), nil
			},
			insertOneCredentialFn: func(_ context.Context, req *auth.Credential) (primitive.ObjectID, error) {
				c := *req
				c.Id = credentialId
				stored = &c
				return credentialId, nil
			},
			findOneCredentialFn: func(_ context.Context, id string) (*auth.Credential, error) {
				if id != credentialId.Hex() {
					t.Errorf("credential id = %q", id)
				}
				return stored, nil
			},
		}
	}

	tests := []struct {
		name    string
		mutate  func(r *fakeAuthRepo)
		wantErr string
	}{
		{name: "success"},
		{
			name: "credential search failed",
			mutate: func(r *fakeAuthRepo) {
				r.credentialSearchFn = func(context.Context, string, *playerPb.CredentialSearchReq) (*playerPb.PlayerProfile, error) {
					return nil, errors.New("error: email or password is incorrect")
				}
			},
			wantErr: "email or password is incorrect",
		},
		{
			name: "insert credential failed",
			mutate: func(r *fakeAuthRepo) {
				r.insertOneCredentialFn = func(context.Context, *auth.Credential) (primitive.ObjectID, error) {
					return primitive.NilObjectID, errors.New("error: insert one player credential failed")
				}
			},
			wantErr: "insert one player credential failed",
		},
		{
			name: "find credential failed",
			mutate: func(r *fakeAuthRepo) {
				r.findOneCredentialFn = func(context.Context, string) (*auth.Credential, error) {
					return nil, errors.New("error: find one player credential failed")
				}
			},
			wantErr: "find one player credential failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepo()
			if tt.mutate != nil {
				tt.mutate(repo)
			}

			res, err := newTestUsecase(repo).Login(context.Background(), cfg, &auth.PlayerLoginReq{
				Email:    "test@inwza.com",
				Password: "123456",
			})
			assertErr(t, err, tt.wantErr)
			if tt.wantErr != "" {
				return
			}

			wantPlayerId := "player:6a927a8535887829348adce9"
			if res.PlayerProfile.Id != wantPlayerId || res.Credential.PlayerId != wantPlayerId {
				t.Fatalf("player id = %q / %q", res.PlayerProfile.Id, res.Credential.PlayerId)
			}
			if res.Credential.Id != credentialId.Hex() {
				t.Fatalf("credential id = %q", res.Credential.Id)
			}
			if res.Credential.RoleCode != 1 {
				t.Fatalf("role code = %d", res.Credential.RoleCode)
			}

			accessClaims, err := jwtauth.ParseToken(cfg.Jwt.AccessSecretKey, res.Credential.AccessToken)
			if err != nil {
				t.Fatalf("access token invalid: %v", err)
			}
			if accessClaims.Subject != "access-token" || accessClaims.PlayerId != wantPlayerId {
				t.Fatalf("access claims = %+v", accessClaims)
			}

			refreshClaims, err := jwtauth.ParseToken(cfg.Jwt.RefreshSecretKey, res.Credential.RefreshToken)
			if err != nil {
				t.Fatalf("refresh token invalid: %v", err)
			}
			if refreshClaims.Subject != "refresh-token" {
				t.Fatalf("refresh subject = %q", refreshClaims.Subject)
			}
		})
	}
}

func TestRefreshToken(t *testing.T) {
	cfg := testConfig()
	credentialId := primitive.NewObjectID()
	playerId := "player:6a927a8535887829348adce9"
	claims := &jwtauth.Claims{PlayerId: playerId, RoleCode: 1}

	validRefresh := jwtauth.NewRefreshToken(cfg.Jwt.RefreshSecretKey, cfg.Jwt.RefreshDuration, claims).SignToken()
	accessAsRefresh := jwtauth.NewAccessToken(cfg.Jwt.RefreshSecretKey, cfg.Jwt.AccessDuration, claims).SignToken()

	type state struct {
		updated *auth.UpdateRefreshTokenReq
	}

	newRepo := func(s *state) *fakeAuthRepo {
		return &fakeAuthRepo{
			findOneCredentialFn: func(_ context.Context, id string) (*auth.Credential, error) {
				c := &auth.Credential{
					Id:           credentialId,
					PlayerId:     playerId,
					RoleCode:     1,
					AccessToken:  "old-access",
					RefreshToken: validRefresh,
				}
				if s.updated != nil {
					c.AccessToken = s.updated.AccessToken
					c.RefreshToken = s.updated.RefreshToken
				}
				return c, nil
			},
			findProfileRefreshFn: func(_ context.Context, _ string, req *playerPb.FindOnePlayerProfileToRefreshReq) (*playerPb.PlayerProfile, error) {
				if req.PlayerId != "6a927a8535887829348adce9" {
					t.Errorf("player id sent to player service = %q", req.PlayerId)
				}
				return testProfile(), nil
			},
			updateCredentialFn: func(_ context.Context, id string, req *auth.UpdateRefreshTokenReq) error {
				if id != credentialId.Hex() {
					t.Errorf("update credential id = %q", id)
				}
				s.updated = req
				return nil
			},
		}
	}

	tests := []struct {
		name         string
		refreshToken string
		mutate       func(r *fakeAuthRepo)
		wantErr      string
	}{
		{name: "success", refreshToken: validRefresh},
		{name: "malformed token", refreshToken: "not-a-jwt", wantErr: "token format is invalid"},
		{
			name:         "wrong secret",
			refreshToken: jwtauth.NewRefreshToken("other-secret", 3600, claims).SignToken(),
			wantErr:      "token is invalid",
		},
		{name: "subject is not refresh-token", refreshToken: accessAsRefresh, wantErr: "token is invalid"},
		{
			name:         "credential not found",
			refreshToken: validRefresh,
			mutate: func(r *fakeAuthRepo) {
				r.findOneCredentialFn = func(context.Context, string) (*auth.Credential, error) {
					return nil, errors.New("error: find one player credential failed")
				}
			},
			wantErr: "find one player credential failed",
		},
		{
			name:         "refresh token does not match stored token",
			refreshToken: validRefresh,
			mutate: func(r *fakeAuthRepo) {
				r.findOneCredentialFn = func(context.Context, string) (*auth.Credential, error) {
					return &auth.Credential{Id: credentialId, RefreshToken: "another-token"}, nil
				}
			},
			wantErr: "refresh token is invalid",
		},
		{
			name:         "player profile not found",
			refreshToken: validRefresh,
			mutate: func(r *fakeAuthRepo) {
				r.findProfileRefreshFn = func(context.Context, string, *playerPb.FindOnePlayerProfileToRefreshReq) (*playerPb.PlayerProfile, error) {
					return nil, errors.New("error: player profile not found")
				}
			},
			wantErr: "player profile not found",
		},
		{
			name:         "update credential failed",
			refreshToken: validRefresh,
			mutate: func(r *fakeAuthRepo) {
				r.updateCredentialFn = func(context.Context, string, *auth.UpdateRefreshTokenReq) error {
					return errors.New("error: player credential not found")
				}
			},
			wantErr: "player credential not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{}
			repo := newRepo(s)
			if tt.mutate != nil {
				tt.mutate(repo)
			}

			res, err := newTestUsecase(repo).RefreshToken(context.Background(), cfg, &auth.RefreshTokenReq{
				CredentialId: credentialId.Hex(),
				RefreshToken: tt.refreshToken,
			})
			assertErr(t, err, tt.wantErr)
			if tt.wantErr != "" {
				return
			}

			if s.updated == nil {
				t.Fatal("expected credential to be updated")
			}
			if s.updated.PlayerId != playerId {
				t.Fatalf("updated player id = %q", s.updated.PlayerId)
			}
			if res.Credential.AccessToken != s.updated.AccessToken || res.Credential.RefreshToken != s.updated.RefreshToken {
				t.Fatal("response must return the updated tokens")
			}
			if _, err := jwtauth.ParseToken(cfg.Jwt.AccessSecretKey, res.Credential.AccessToken); err != nil {
				t.Fatalf("new access token invalid: %v", err)
			}
			newRefreshClaims, err := jwtauth.ParseToken(cfg.Jwt.RefreshSecretKey, res.Credential.RefreshToken)
			if err != nil {
				t.Fatalf("new refresh token invalid: %v", err)
			}
			oldClaims, _ := jwtauth.ParseToken(cfg.Jwt.RefreshSecretKey, validRefresh)
			if !newRefreshClaims.ExpiresAt.Equal(oldClaims.ExpiresAt.Time) {
				t.Fatal("reloaded refresh token must keep the original expiry")
			}
		})
	}
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name        string
		findErr     error
		deleteCount int64
		deleteErr   error
		wantCount   int64
		wantErr     string
	}{
		{name: "success", deleteCount: 1, wantCount: 1},
		{name: "credential lookup fails but still deletes", findErr: errors.New("not found"), deleteCount: 1, wantCount: 1},
		{name: "delete failed", deleteErr: errors.New("error: player credential not found"), wantErr: "player credential not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteCalled := false
			repo := &fakeAuthRepo{
				findOneCredentialFn: func(context.Context, string) (*auth.Credential, error) {
					if tt.findErr != nil {
						return nil, tt.findErr
					}
					return &auth.Credential{AccessToken: "access"}, nil
				},
				deleteCredentialFn: func(_ context.Context, id string) (int64, error) {
					deleteCalled = true
					if id != "cred-1" {
						t.Errorf("delete id = %q", id)
					}
					return tt.deleteCount, tt.deleteErr
				},
			}

			count, err := newTestUsecase(repo).Logout(context.Background(), "cred-1")
			assertErr(t, err, tt.wantErr)
			if !deleteCalled {
				t.Fatal("expected delete to be called")
			}
			if count != tt.wantCount {
				t.Fatalf("count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestAccessTokenSearch(t *testing.T) {
	tests := []struct {
		name       string
		credential *auth.Credential
		repoErr    error
		wantValid  bool
		wantErr    string
	}{
		{name: "valid token", credential: &auth.Credential{AccessToken: "token"}, wantValid: true},
		{name: "repo error", repoErr: errors.New("error: acces token not found"), wantErr: "acces token not found"},
		{name: "nil credential", wantErr: "access token is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeAuthRepo{
				findOneAccessTokenFn: func(_ context.Context, token string) (*auth.Credential, error) {
					if token != "token" {
						t.Errorf("token = %q", token)
					}
					return tt.credential, tt.repoErr
				},
			}

			res, err := newTestUsecase(repo).AccessTokenSearch(context.Background(), "token")
			assertErr(t, err, tt.wantErr)
			if res == nil || res.IsValid != tt.wantValid {
				t.Fatalf("res = %+v, want valid=%v", res, tt.wantValid)
			}
		})
	}
}

func TestRolesCount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeAuthRepo{rolesCountFn: func(context.Context) (int64, error) { return 2, nil }}
		res, err := newTestUsecase(repo).RolesCount(context.Background())
		assertErr(t, err, "")
		if res.Count != 2 {
			t.Fatalf("count = %d", res.Count)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &fakeAuthRepo{rolesCountFn: func(context.Context) (int64, error) {
			return -1, errors.New("error: roles count failed")
		}}
		res, err := newTestUsecase(repo).RolesCount(context.Background())
		assertErr(t, err, "roles count failed")
		if res != nil {
			t.Fatalf("res = %+v, want nil", res)
		}
	})
}

func TestFormatPlayerId(t *testing.T) {
	for _, in := range []string{"abc", "player:abc"} {
		if got := formatPlayerId(in); got != "player:abc" {
			t.Fatalf("formatPlayerId(%q) = %q", in, got)
		}
	}
}
