package authHandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/auth"
	authUsecase "github.com/watcharaphong99/InwzaShop/modules/auth/authUseCase"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"github.com/watcharaphong99/InwzaShop/pkg/rediscon"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// memAuthRepo stands in for Mongo (credentials) and the player gRPC service (profiles).
type memAuthRepo struct {
	mu          sync.Mutex
	credentials map[string]*auth.Credential
	players     map[string]*playerPb.PlayerProfile
	passwords   map[string]string
}

func newMemAuthRepo() *memAuthRepo {
	profile := &playerPb.PlayerProfile{
		Id:        "6a927a8535887829348adce9",
		Email:     "test@inwza.com",
		Username:  "tester",
		RoleCode:  0,
		CreatedAt: "2026-01-01T10:00:00+07:00",
		UpdatedAt: "2026-01-01T10:00:00+07:00",
	}
	return &memAuthRepo{
		credentials: map[string]*auth.Credential{},
		players:     map[string]*playerPb.PlayerProfile{profile.Email: profile},
		passwords:   map[string]string{profile.Email: "123456"},
	}
}

func (r *memAuthRepo) InsertOnePlayerCredential(_ context.Context, req *auth.Credential) (primitive.ObjectID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *req
	c.Id = primitive.NewObjectID()
	r.credentials[c.Id.Hex()] = &c
	return c.Id, nil
}

func (r *memAuthRepo) CredentialSearch(_ context.Context, _ string, req *playerPb.CredentialSearchReq) (*playerPb.PlayerProfile, error) {
	p, ok := r.players[req.Email]
	if !ok || r.passwords[req.Email] != req.Password {
		return nil, errors.New("error: email or password is incorrect")
	}
	return p, nil
}

func (r *memAuthRepo) FindOnePlayerCredential(_ context.Context, credentialId string) (*auth.Credential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.credentials[credentialId]
	if !ok {
		return nil, errors.New("error: find one player credential failed")
	}
	cp := *c
	return &cp, nil
}

func (r *memAuthRepo) FindOnePlayerProfileToRefresh(_ context.Context, _ string, req *playerPb.FindOnePlayerProfileToRefreshReq) (*playerPb.PlayerProfile, error) {
	for _, p := range r.players {
		if p.Id == req.PlayerId {
			return p, nil
		}
	}
	return nil, errors.New("error: player profile not found")
}

func (r *memAuthRepo) UpdateOnePlayerCredential(_ context.Context, credentialId string, req *auth.UpdateRefreshTokenReq) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.credentials[credentialId]
	if !ok {
		return errors.New("error: player credential not found")
	}
	c.PlayerId = req.PlayerId
	c.AccessToken = req.AccessToken
	c.RefreshToken = req.RefreshToken
	c.UpdatedAt = req.UpdatedAt
	return nil
}

func (r *memAuthRepo) DeleteOnePlayerCredential(_ context.Context, credentialId string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.credentials[credentialId]; !ok {
		return 0, errors.New("error: player credential not found")
	}
	delete(r.credentials, credentialId)
	return 1, nil
}

func (r *memAuthRepo) FindOneAccessToken(_ context.Context, accessToken string) (*auth.Credential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.credentials {
		if c.AccessToken == accessToken {
			cp := *c
			return &cp, nil
		}
	}
	return nil, errors.New("error: acces token not found")
}

func (r *memAuthRepo) RolesCount(context.Context) (int64, error) {
	return 2, nil
}

func postJSON(t *testing.T, e *echo.Echo, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAuthFlowLoginRefreshLogout(t *testing.T) {
	cfg := &config.Config{
		Jwt: config.Jwt{
			AccessSecretKey:  "access-secret",
			RefreshSecretKey: "refresh-secret",
			AccessDuration:   60,
			RefreshDuration:  3600,
		},
	}
	repo := newMemAuthRepo()
	uc := authUsecase.NewAuthUseCase(repo, &rediscon.Client{}, cfg.Jwt.AccessDuration)
	h := NewAuthHandlerService(cfg, uc)

	e := echo.New()
	e.POST("/login", h.Login)
	e.POST("/refresh-token", h.RefreshToken)
	e.POST("/logout", h.Logout)

	if rec := postJSON(t, e, "/login", `{"email":"test@inwza.com","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("login with wrong password: status = %d", rec.Code)
	}

	rec := postJSON(t, e, "/login", `{"email":"test@inwza.com","password":"123456"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status = %d, body %s", rec.Code, rec.Body.String())
	}
	var login auth.ProfileIntercepter
	if err := json.Unmarshal(rec.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	if login.Id != "player:6a927a8535887829348adce9" {
		t.Fatalf("login player id = %q", login.Id)
	}
	credentialId := login.Credential.Id

	accessRes, err := uc.AccessTokenSearch(context.Background(), login.Credential.AccessToken)
	if err != nil || !accessRes.IsValid {
		t.Fatalf("access token from login should be valid: %+v, %v", accessRes, err)
	}

	refreshBody, _ := json.Marshal(auth.RefreshTokenReq{CredentialId: credentialId, RefreshToken: login.Credential.RefreshToken})
	rec = postJSON(t, e, "/refresh-token", string(refreshBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: status = %d, body %s", rec.Code, rec.Body.String())
	}
	var refreshed auth.ProfileIntercepter
	if err := json.Unmarshal(rec.Body.Bytes(), &refreshed); err != nil {
		t.Fatal(err)
	}
	if refreshed.Credential.Id != credentialId {
		t.Fatalf("refresh must keep credential id: %q vs %q", refreshed.Credential.Id, credentialId)
	}
	if _, err := jwtauth.ParseToken(cfg.Jwt.AccessSecretKey, refreshed.Credential.AccessToken); err != nil {
		t.Fatalf("refreshed access token invalid: %v", err)
	}

	logoutBody := `{"credential_id":"` + credentialId + `"}`
	rec = postJSON(t, e, "/logout", logoutBody)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Deleted count: 1") {
		t.Fatalf("logout: status = %d, body %s", rec.Code, rec.Body.String())
	}

	if rec := postJSON(t, e, "/logout", logoutBody); rec.Code != http.StatusNotFound {
		t.Fatalf("second logout: status = %d, want 404", rec.Code)
	}

	if rec := postJSON(t, e, "/refresh-token", string(refreshBody)); rec.Code != http.StatusBadRequest {
		t.Fatalf("refresh after logout: status = %d, want 400", rec.Code)
	}
}
