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

func (r *memPlayerRepo) GetOffset(context.Context) (int64, error) {
	return -1, nil
}

func (r *memPlayerRepo) UpsertOffset(context.Context, int64) error {
	return nil
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
	for _, existing := range r.players {
		if existing.Email == req.Email || existing.Username == req.Username {
			return primitive.NilObjectID, errors.New("error: email or username already exist")
		}
	}
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
	if req.EventId != "" {
		for _, tx := range r.transactions {
			if tx.EventId == req.EventId {
				if tx.PlayerId != req.PlayerId || tx.Amount != req.Amount {
					return errors.New("error: event_id already used")
				}
				return nil
			}
		}
	}
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

func TestPlayerFlowAddMoneyEventId(t *testing.T) {
	repo := newMemPlayerRepo()
	uc := playerUsecase.NewPlayerUsecase(repo)
	req := &player.CreatePlayerTransactionReq{PlayerId: "player:abc", Amount: 100, EventId: "pay-1"}

	first, err := uc.AddPlayerMoney(context.Background(), req)
	if err != nil {
		t.Fatalf("first add: %v", err)
	}
	if first.Balance != 100 {
		t.Fatalf("first balance = %v, want 100", first.Balance)
	}

	replay, err := uc.AddPlayerMoney(context.Background(), req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replay.Balance != 100 {
		t.Fatalf("replay balance = %v, want 100", replay.Balance)
	}

	_, err = uc.AddPlayerMoney(context.Background(), &player.CreatePlayerTransactionReq{
		PlayerId: "player:xyz", Amount: 100, EventId: "pay-1",
	})
	if err == nil || err.Error() != "error: event_id already used" {
		t.Fatalf("other player replay err = %v", err)
	}

	_, err = uc.AddPlayerMoney(context.Background(), &player.CreatePlayerTransactionReq{
		PlayerId: "player:abc", Amount: -100, EventId: "pay-1",
	})
	if err == nil || err.Error() != "error: event_id already used" {
		t.Fatalf("different amount replay err = %v", err)
	}

	other, err := uc.AddPlayerMoney(context.Background(), &player.CreatePlayerTransactionReq{
		PlayerId: "player:abc", Amount: 50, EventId: "pay-2",
	})
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	if other.Balance != 150 {
		t.Fatalf("new event balance = %v, want 150", other.Balance)
	}
}
