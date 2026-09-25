package authHandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/auth"
	authPb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	"github.com/watcharaphong99/InwzaShop/modules/player"
	"github.com/watcharaphong99/InwzaShop/pkg/response"
)

type fakeAuthUsecase struct {
	loginFn             func(ctx context.Context, cfg *config.Config, req *auth.PlayerLoginReq) (*auth.ProfileIntercepter, error)
	refreshTokenFn      func(ctx context.Context, cfg *config.Config, req *auth.RefreshTokenReq) (*auth.ProfileIntercepter, error)
	logoutFn            func(ctx context.Context, credentialId string) (int64, error)
	accessTokenSearchFn func(ctx context.Context, accessToken string) (*authPb.AccessTokenSearchRes, error)
	rolesCountFn        func(ctx context.Context) (*authPb.RolesCountRes, error)
}

func (f *fakeAuthUsecase) Login(ctx context.Context, cfg *config.Config, req *auth.PlayerLoginReq) (*auth.ProfileIntercepter, error) {
	return f.loginFn(ctx, cfg, req)
}

func (f *fakeAuthUsecase) RefreshToken(ctx context.Context, cfg *config.Config, req *auth.RefreshTokenReq) (*auth.ProfileIntercepter, error) {
	return f.refreshTokenFn(ctx, cfg, req)
}

func (f *fakeAuthUsecase) Logout(ctx context.Context, credentialId string) (int64, error) {
	return f.logoutFn(ctx, credentialId)
}

func (f *fakeAuthUsecase) AccessTokenSearch(ctx context.Context, accessToken string) (*authPb.AccessTokenSearchRes, error) {
	return f.accessTokenSearchFn(ctx, accessToken)
}

func (f *fakeAuthUsecase) RolesCount(ctx context.Context) (*authPb.RolesCountRes, error) {
	return f.rolesCountFn(ctx)
}

func doJSON(t *testing.T, handler echo.HandlerFunc, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	return rec
}

func decodeMsg(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var msg response.MsgResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return msg.Message
}

func profileRes() *auth.ProfileIntercepter {
	return &auth.ProfileIntercepter{
		PlayerProfile: &player.PlayerProfile{Id: "player:abc", Email: "test@inwza.com"},
		Credential:    &auth.CredentialRes{Id: "cred-1", AccessToken: "access", RefreshToken: "refresh"},
	}
}

func TestLoginHandler(t *testing.T) {
	cfg := &config.Config{}

	tests := []struct {
		name       string
		body       string
		ucErr      error
		wantStatus int
		wantMsg    string
	}{
		{name: "success", body: `{"email":"test@inwza.com","password":"123456"}`, wantStatus: http.StatusOK},
		{name: "invalid json", body: `{`, wantStatus: http.StatusBadRequest, wantMsg: "error: bad request"},
		{name: "missing password", body: `{"email":"test@inwza.com"}`, wantStatus: http.StatusBadRequest, wantMsg: "password"},
		{name: "invalid email", body: `{"email":"not-email","password":"123456"}`, wantStatus: http.StatusBadRequest, wantMsg: "email"},
		{
			name:       "usecase error",
			body:       `{"email":"test@inwza.com","password":"wrong"}`,
			ucErr:      errors.New("error: email or password is incorrect"),
			wantStatus: http.StatusUnauthorized,
			wantMsg:    "email or password is incorrect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &fakeAuthUsecase{loginFn: func(_ context.Context, gotCfg *config.Config, req *auth.PlayerLoginReq) (*auth.ProfileIntercepter, error) {
				if gotCfg != cfg {
					t.Error("handler must pass its config to usecase")
				}
				if tt.ucErr != nil {
					return nil, tt.ucErr
				}
				return profileRes(), nil
			}}

			rec := doJSON(t, NewAuthHandlerService(cfg, uc).Login, http.MethodPost, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantMsg != "" {
				if msg := decodeMsg(t, rec); !strings.Contains(msg, tt.wantMsg) {
					t.Fatalf("message = %q, want contains %q", msg, tt.wantMsg)
				}
				return
			}

			var res auth.ProfileIntercepter
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
				t.Fatal(err)
			}
			if res.Credential == nil || res.Credential.AccessToken != "access" || res.Id != "player:abc" {
				t.Fatalf("body = %s", rec.Body.String())
			}
		})
	}
}

func TestRefreshTokenHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		ucErr      error
		wantStatus int
	}{
		{name: "success", body: `{"credential_id":"cred-1","refresh_token":"refresh"}`, wantStatus: http.StatusOK},
		{name: "missing refresh token", body: `{"credential_id":"cred-1"}`, wantStatus: http.StatusBadRequest},
		{
			name:       "usecase error",
			body:       `{"credential_id":"cred-1","refresh_token":"bad"}`,
			ucErr:      errors.New("error: token is invalid"),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &fakeAuthUsecase{refreshTokenFn: func(_ context.Context, _ *config.Config, req *auth.RefreshTokenReq) (*auth.ProfileIntercepter, error) {
				if req.CredentialId != "cred-1" {
					t.Errorf("credential id = %q", req.CredentialId)
				}
				if tt.ucErr != nil {
					return nil, tt.ucErr
				}
				return profileRes(), nil
			}}

			rec := doJSON(t, NewAuthHandlerService(&config.Config{}, uc).RefreshToken, http.MethodPost, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestLogoutHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		count      int64
		ucErr      error
		wantStatus int
		wantMsg    string
	}{
		{name: "success", body: `{"credential_id":"cred-1"}`, count: 1, wantStatus: http.StatusOK, wantMsg: "Deleted count: 1"},
		{name: "missing credential id", body: `{}`, wantStatus: http.StatusBadRequest, wantMsg: "credentialid"},
		{
			name:       "credential not found",
			body:       `{"credential_id":"cred-1"}`,
			ucErr:      errors.New("error: player credential not found"),
			wantStatus: http.StatusNotFound,
			wantMsg:    "player credential not found",
		},
		{
			name:       "invalid credential id",
			body:       `{"credential_id":"cred-1"}`,
			ucErr:      errors.New("error: credential id is invalid"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "credential id is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &fakeAuthUsecase{logoutFn: func(_ context.Context, id string) (int64, error) {
				if id != "cred-1" {
					t.Errorf("credential id = %q", id)
				}
				return tt.count, tt.ucErr
			}}

			rec := doJSON(t, NewAuthHandlerService(&config.Config{}, uc).Logout, http.MethodPost, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if msg := decodeMsg(t, rec); !strings.Contains(msg, tt.wantMsg) {
				t.Fatalf("message = %q, want contains %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestAuthGrpcHandler(t *testing.T) {
	uc := &fakeAuthUsecase{
		accessTokenSearchFn: func(_ context.Context, token string) (*authPb.AccessTokenSearchRes, error) {
			return &authPb.AccessTokenSearchRes{IsValid: token == "good"}, nil
		},
		rolesCountFn: func(context.Context) (*authPb.RolesCountRes, error) {
			return &authPb.RolesCountRes{Count: 2}, nil
		},
	}
	h := NewAuthGrpcHandler(uc)

	res, err := h.AccessTokenSearch(context.Background(), &authPb.AccessTokenSearchReq{AccessToken: "good"})
	if err != nil || !res.IsValid {
		t.Fatalf("AccessTokenSearch = %+v, %v", res, err)
	}

	count, err := h.RolesCount(context.Background(), &authPb.RolesCountReq{})
	if err != nil || count.Count != 2 {
		t.Fatalf("RolesCount = %+v, %v", count, err)
	}
}
