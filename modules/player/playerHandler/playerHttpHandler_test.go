package playerHandler

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
	"github.com/watcharaphong99/InwzaShop/modules/player"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/pkg/response"
)

type fakePlayerUsecase struct {
	createPlayerFn       func(ctx context.Context, req *player.CreatePlayerReq) (*player.PlayerProfile, error)
	findProfileFn        func(ctx context.Context, playerId string) (*player.PlayerProfile, error)
	addMoneyFn           func(ctx context.Context, req *player.CreatePlayerTransactionReq) (*player.PlayerSavingAccount, error)
	getSavingAccountFn   func(ctx context.Context, playerId string) (*player.PlayerSavingAccount, error)
	findCredentialFn     func(ctx context.Context, email, password string) (*playerPb.PlayerProfile, error)
	findProfileRefreshFn func(ctx context.Context, playerId string) (*playerPb.PlayerProfile, error)
}

func (f *fakePlayerUsecase) CreatePlayer(ctx context.Context, req *player.CreatePlayerReq) (*player.PlayerProfile, error) {
	return f.createPlayerFn(ctx, req)
}

func (f *fakePlayerUsecase) FindOnePlayerProfile(ctx context.Context, playerId string) (*player.PlayerProfile, error) {
	return f.findProfileFn(ctx, playerId)
}

func (f *fakePlayerUsecase) AddPlayerMoney(ctx context.Context, req *player.CreatePlayerTransactionReq) (*player.PlayerSavingAccount, error) {
	return f.addMoneyFn(ctx, req)
}

func (f *fakePlayerUsecase) GetPlayerSavingAccount(ctx context.Context, playerId string) (*player.PlayerSavingAccount, error) {
	return f.getSavingAccountFn(ctx, playerId)
}

func (f *fakePlayerUsecase) FindOnePlayerCredential(ctx context.Context, email, password string) (*playerPb.PlayerProfile, error) {
	return f.findCredentialFn(ctx, email, password)
}

func (f *fakePlayerUsecase) FindOnePlayerProfileToRefresh(ctx context.Context, playerId string) (*playerPb.PlayerProfile, error) {
	return f.findProfileRefreshFn(ctx, playerId)
}

type reqOpts struct {
	body       string
	paramValue string
	playerId   any
}

func serve(t *testing.T, handler echo.HandlerFunc, method string, o reqOpts) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/", strings.NewReader(o.body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if o.paramValue != "" {
		c.SetParamNames("player_id")
		c.SetParamValues(o.paramValue)
	}
	if o.playerId != nil {
		c.Set("player_id", o.playerId)
	}
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	return rec
}

func msgOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var msg response.MsgResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return msg.Message
}

func newHandler(uc *fakePlayerUsecase) PlayerHttpHandlerService {
	return NewPlayerHttpHandlerService(&config.Config{}, uc)
}

func TestCreatePlayerHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		ucErr      error
		wantStatus int
		wantMsg    string
	}{
		{name: "success", body: `{"email":"a@inwza.com","password":"123456","username":"a"}`, wantStatus: http.StatusCreated},
		{name: "invalid json", body: `{`, wantStatus: http.StatusBadRequest, wantMsg: "bad request"},
		{name: "missing username", body: `{"email":"a@inwza.com","password":"123456"}`, wantStatus: http.StatusBadRequest, wantMsg: "username"},
		{
			name:       "duplicate player",
			body:       `{"email":"a@inwza.com","password":"123456","username":"a"}`,
			ucErr:      errors.New("error: email or username already exist"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "already exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &fakePlayerUsecase{createPlayerFn: func(_ context.Context, req *player.CreatePlayerReq) (*player.PlayerProfile, error) {
				if tt.ucErr != nil {
					return nil, tt.ucErr
				}
				return &player.PlayerProfile{Id: "abc", Email: req.Email, Username: req.Username}, nil
			}}

			rec := serve(t, newHandler(uc).CreatePlayer, http.MethodPost, reqOpts{body: tt.body})
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantMsg != "" {
				if msg := msgOf(t, rec); !strings.Contains(msg, tt.wantMsg) {
					t.Fatalf("message = %q, want contains %q", msg, tt.wantMsg)
				}
				return
			}
			var res player.PlayerProfile
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || res.Email != "a@inwza.com" {
				t.Fatalf("body = %s (%v)", rec.Body.String(), err)
			}
		})
	}
}

func TestFindOnePlayerProfileHandler(t *testing.T) {
	tests := []struct {
		name       string
		param      string
		ucErr      error
		wantStatus int
	}{
		{name: "raw id", param: "abc", wantStatus: http.StatusOK},
		{name: "prefixed id is trimmed", param: "player:abc", wantStatus: http.StatusOK},
		{name: "not found", param: "abc", ucErr: errors.New("error: player profile not found"), wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &fakePlayerUsecase{findProfileFn: func(_ context.Context, id string) (*player.PlayerProfile, error) {
				if id != "abc" {
					t.Errorf("usecase got player id %q, want %q", id, "abc")
				}
				if tt.ucErr != nil {
					return nil, tt.ucErr
				}
				return &player.PlayerProfile{Id: id}, nil
			}}

			rec := serve(t, newHandler(uc).FindOnePlayerProfile, http.MethodGet, reqOpts{paramValue: tt.param})
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestAddPlayerMoneyHandler(t *testing.T) {
	validBody := `{"player_id":"ignored","amount":100}`

	tests := []struct {
		name       string
		body       string
		playerId   any
		ucErr      error
		wantStatus int
		wantMsg    string
	}{
		{name: "success uses player id from context", body: validBody, playerId: "abc", wantStatus: http.StatusCreated},
		{name: "missing amount", body: `{"player_id":"x"}`, playerId: "abc", wantStatus: http.StatusBadRequest, wantMsg: "amount"},
		{name: "no player id in context", body: validBody, wantStatus: http.StatusUnauthorized, wantMsg: "player_id is required"},
		{name: "player id wrong type", body: validBody, playerId: 123, wantStatus: http.StatusUnauthorized, wantMsg: "player_id is required"},
		{name: "player id is only prefix", body: validBody, playerId: "player:", wantStatus: http.StatusBadRequest, wantMsg: "player_id is required"},
		{
			name:       "usecase error",
			body:       validBody,
			playerId:   "abc",
			ucErr:      errors.New("error: insert one player transaction failed"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "insert one player transaction failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &fakePlayerUsecase{addMoneyFn: func(_ context.Context, req *player.CreatePlayerTransactionReq) (*player.PlayerSavingAccount, error) {
				if req.PlayerId != "player:abc" {
					t.Errorf("usecase got player id %q", req.PlayerId)
				}
				if tt.ucErr != nil {
					return nil, tt.ucErr
				}
				return &player.PlayerSavingAccount{PlayerId: req.PlayerId, Balance: req.Amount}, nil
			}}

			rec := serve(t, newHandler(uc).AddPlayerMoney, http.MethodPost, reqOpts{body: tt.body, playerId: tt.playerId})
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantMsg != "" {
				if msg := msgOf(t, rec); !strings.Contains(msg, tt.wantMsg) {
					t.Fatalf("message = %q, want contains %q", msg, tt.wantMsg)
				}
			}
		})
	}
}

func TestGetPlayerSavingAccountHandler(t *testing.T) {
	tests := []struct {
		name       string
		playerId   any
		ucErr      error
		wantStatus int
	}{
		{name: "success", playerId: "abc", wantStatus: http.StatusOK},
		{name: "prefixed id", playerId: "player:abc", wantStatus: http.StatusOK},
		{name: "no player id", wantStatus: http.StatusUnauthorized},
		{name: "usecase error", playerId: "abc", ucErr: errors.New("error: not found"), wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &fakePlayerUsecase{getSavingAccountFn: func(_ context.Context, id string) (*player.PlayerSavingAccount, error) {
				if id != "player:abc" {
					t.Errorf("usecase got player id %q", id)
				}
				if tt.ucErr != nil {
					return nil, tt.ucErr
				}
				return &player.PlayerSavingAccount{PlayerId: id, Balance: 10}, nil
			}}

			rec := serve(t, newHandler(uc).GetPlayerSavingAccount, http.MethodGet, reqOpts{playerId: tt.playerId})
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestPlayerGrpcHandler(t *testing.T) {
	uc := &fakePlayerUsecase{
		findCredentialFn: func(_ context.Context, email, password string) (*playerPb.PlayerProfile, error) {
			if email != "a@inwza.com" || password != "123456" {
				return nil, errors.New("error: password is invalid")
			}
			return &playerPb.PlayerProfile{Id: "abc", Email: email}, nil
		},
		findProfileRefreshFn: func(_ context.Context, id string) (*playerPb.PlayerProfile, error) {
			return &playerPb.PlayerProfile{Id: id}, nil
		},
	}
	h := NewPlayerGrpcHandler(uc)

	res, err := h.CredentialSearch(context.Background(), &playerPb.CredentialSearchReq{Email: "a@inwza.com", Password: "123456"})
	if err != nil || res.Id != "abc" {
		t.Fatalf("CredentialSearch = %+v, %v", res, err)
	}

	if _, err := h.CredentialSearch(context.Background(), &playerPb.CredentialSearchReq{Email: "a@inwza.com", Password: "bad"}); err == nil {
		t.Fatal("expected error for wrong password")
	}

	profile, err := h.FindOnePlayerProfileToRefresh(context.Background(), &playerPb.FindOnePlayerProfileToRefreshReq{PlayerId: "abc"})
	if err != nil || profile.Id != "abc" {
		t.Fatalf("FindOnePlayerProfileToRefresh = %+v, %v", profile, err)
	}
}
