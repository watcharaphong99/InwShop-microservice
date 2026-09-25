package playerHandler

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
	"github.com/watcharaphong99/InwzaShop/modules/player"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	playerUsecase "github.com/watcharaphong99/InwzaShop/modules/player/playerUseCase"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memPlayerRepo struct {
	mu           sync.Mutex
	players      map[string]*player.Player
	transactions []*player.PlayerTransaction
}

func newMemPlayerRepo() *memPlayerRepo {
	return &memPlayerRepo{players: map[string]*player.Player{}}
}

func (r *memPlayerRepo) IsUniquePlayer(_ context.Context, email, username string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.players {
		if p.Email == email || p.Username == username {
			return false
		}
	}
	return true
}

func (r *memPlayerRepo) InsertOnePlayer(_ context.Context, req *player.Player) (primitive.ObjectID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := *req
	p.Id = primitive.NewObjectID()
	r.players[p.Id.Hex()] = &p
	return p.Id, nil
}

func (r *memPlayerRepo) FindOnePlayerProfine(_ context.Context, playerId string) (*player.PlayerProfileBson, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[playerId]
	if !ok {
		return nil, errors.New("error: player profile not found")
	}
	return &player.PlayerProfileBson{Id: p.Id, Email: p.Email, Username: p.Username, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}, nil
}

func (r *memPlayerRepo) InsertOnePlayerTranscation(_ context.Context, req *player.PlayerTransaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transactions = append(r.transactions, req)
	return nil
}

func (r *memPlayerRepo) GetPlayerSavingAccount(_ context.Context, playerId string) (*player.PlayerSavingAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := &player.PlayerSavingAccount{PlayerId: playerId}
	for _, tx := range r.transactions {
		if tx.PlayerId == playerId {
			res.Balance += tx.Amount
		}
	}
	return res, nil
}

func (r *memPlayerRepo) FindOnePlayerCredential(_ context.Context, email string) (*player.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.players {
		if p.Email == email {
			return p, nil
		}
	}
	return nil, errors.New("error: email is invalid")
}

func (r *memPlayerRepo) FindOnePlayerProfileTokenRefresh(_ context.Context, playerId string) (*player.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[playerId]
	if !ok {
		return nil, errors.New("error: player profile not found")
	}
	return p, nil
}

func TestPlayerFlowCreateThenFindProfile(t *testing.T) {
	repo := newMemPlayerRepo()
	uc := playerUsecase.NewPlayerUsecase(repo)
	h := NewPlayerHttpHandlerService(&config.Config{}, uc)
	grpc := NewPlayerGrpcHandler(uc)

	e := echo.New()
	e.POST("/players", h.CreatePlayer)
	e.GET("/players/:player_id", h.FindOnePlayerProfile)

	body := `{"email":"new@inwza.com","password":"123456","username":"newbie"}`
	req := httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body.String())
	}
	var created player.PlayerProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Id == "" || created.Email != "new@inwza.com" {
		t.Fatalf("created = %+v", created)
	}

	req = httptest.NewRequest(http.MethodPost, "/players", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate create: status = %d, want 400", rec.Code)
	}

	for _, id := range []string{created.Id, "player:" + created.Id} {
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/players/"+id, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("find %q: status = %d, body %s", id, rec.Code, rec.Body.String())
		}
		var found player.PlayerProfile
		if err := json.Unmarshal(rec.Body.Bytes(), &found); err != nil {
			t.Fatal(err)
		}
		if found.Id != created.Id || found.Username != "newbie" {
			t.Fatalf("found = %+v", found)
		}
	}

	profile, err := grpc.CredentialSearch(context.Background(), &playerPb.CredentialSearchReq{Email: "new@inwza.com", Password: "123456"})
	if err != nil || profile.Id != created.Id {
		t.Fatalf("login via gRPC with stored bcrypt hash: %+v, %v", profile, err)
	}
}
