package playerUsecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/watcharaphong99/InwzaShop/modules/player"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

var errNotImplemented = errors.New("fake: not implemented")

type fakePlayerRepo struct {
	isUniqueFn           func(ctx context.Context, email, username string) bool
	insertOnePlayerFn    func(ctx context.Context, req *player.Player) (primitive.ObjectID, error)
	findProfileFn        func(ctx context.Context, playerId string) (*player.PlayerProfileBson, error)
	insertTransactionFn  func(ctx context.Context, req *player.PlayerTransaction) error
	getSavingAccountFn   func(ctx context.Context, playerId string) (*player.PlayerSavingAccount, error)
	findCredentialFn     func(ctx context.Context, email string) (*player.Player, error)
	findProfileRefreshFn func(ctx context.Context, playerId string) (*player.Player, error)
}

func (f *fakePlayerRepo) IsUniquePlayer(ctx context.Context, email, username string) bool {
	if f.isUniqueFn == nil {
		return false
	}
	return f.isUniqueFn(ctx, email, username)
}

func (f *fakePlayerRepo) InsertOnePlayer(ctx context.Context, req *player.Player) (primitive.ObjectID, error) {
	if f.insertOnePlayerFn == nil {
		return primitive.NilObjectID, errNotImplemented
	}
	return f.insertOnePlayerFn(ctx, req)
}

func (f *fakePlayerRepo) FindOnePlayerProfine(ctx context.Context, playerId string) (*player.PlayerProfileBson, error) {
	if f.findProfileFn == nil {
		return nil, errNotImplemented
	}
	return f.findProfileFn(ctx, playerId)
}

func (f *fakePlayerRepo) InsertOnePlayerTranscation(ctx context.Context, req *player.PlayerTransaction) error {
	if f.insertTransactionFn == nil {
		return errNotImplemented
	}
	return f.insertTransactionFn(ctx, req)
}

func (f *fakePlayerRepo) GetPlayerSavingAccount(ctx context.Context, playerId string) (*player.PlayerSavingAccount, error) {
	if f.getSavingAccountFn == nil {
		return nil, errNotImplemented
	}
	return f.getSavingAccountFn(ctx, playerId)
}

func (f *fakePlayerRepo) FindOnePlayerCredential(ctx context.Context, email string) (*player.Player, error) {
	if f.findCredentialFn == nil {
		return nil, errNotImplemented
	}
	return f.findCredentialFn(ctx, email)
}

func (f *fakePlayerRepo) FindOnePlayerProfileTokenRefresh(ctx context.Context, playerId string) (*player.Player, error) {
	if f.findProfileRefreshFn == nil {
		return nil, errNotImplemented
	}
	return f.findProfileRefreshFn(ctx, playerId)
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

func testPlayer(t *testing.T, password string, roles ...player.PlayerRole) *player.Player {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &player.Player{
		Id:          primitive.NewObjectID(),
		Email:       "test@inwza.com",
		Password:    string(hashed),
		Username:    "tester",
		CreatedAt:   time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC),
		PlayerRoles: roles,
	}
}

func TestCreatePlayer(t *testing.T) {
	req := &player.CreatePlayerReq{Email: "test@inwza.com", Password: "123456", Username: "tester"}
	playerId := primitive.NewObjectID()

	tests := []struct {
		name      string
		unique    bool
		insertErr error
		findErr   error
		wantErr   string
	}{
		{name: "success", unique: true},
		{name: "email or username already exist", unique: false, wantErr: "already exist"},
		{name: "insert failed", unique: true, insertErr: errors.New("error: insert one player failed"), wantErr: "insert one player failed"},
		{name: "find profile failed", unique: true, findErr: errors.New("error: player profile not found"), wantErr: "player profile not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inserted *player.Player
			findCalled := false
			repo := &fakePlayerRepo{
				isUniqueFn: func(_ context.Context, email, username string) bool {
					if email != req.Email || username != req.Username {
						t.Errorf("unique check got %q %q", email, username)
					}
					return tt.unique
				},
				insertOnePlayerFn: func(_ context.Context, p *player.Player) (primitive.ObjectID, error) {
					inserted = p
					return playerId, tt.insertErr
				},
				findProfileFn: func(_ context.Context, id string) (*player.PlayerProfileBson, error) {
					findCalled = true
					if id != playerId.Hex() {
						t.Errorf("find profile id = %q", id)
					}
					if tt.findErr != nil {
						return nil, tt.findErr
					}
					return &player.PlayerProfileBson{Id: playerId, Email: inserted.Email, Username: inserted.Username}, nil
				},
			}

			res, err := NewPlayerUsecase(repo).CreatePlayer(context.Background(), req)
			assertErr(t, err, tt.wantErr)

			if tt.insertErr != nil && findCalled {
				t.Fatal("must not look up profile when insert failed")
			}
			if tt.wantErr != "" {
				return
			}

			if inserted.Password == req.Password {
				t.Fatal("password must be hashed")
			}
			if err := bcrypt.CompareHashAndPassword([]byte(inserted.Password), []byte(req.Password)); err != nil {
				t.Fatalf("stored hash does not match password: %v", err)
			}
			if len(inserted.PlayerRoles) != 1 || inserted.PlayerRoles[0].RoleTitle != "player" || inserted.PlayerRoles[0].RoleCode != 0 {
				t.Fatalf("default roles = %+v", inserted.PlayerRoles)
			}
			if res.Id != playerId.Hex() || res.Email != req.Email || res.Username != req.Username {
				t.Fatalf("res = %+v", res)
			}
		})
	}
}

func TestFindOnePlayerProfile(t *testing.T) {
	id := primitive.NewObjectID()
	createdAt := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)

	t.Run("success converts to Asia/Bangkok", func(t *testing.T) {
		repo := &fakePlayerRepo{findProfileFn: func(context.Context, string) (*player.PlayerProfileBson, error) {
			return &player.PlayerProfileBson{Id: id, Email: "a@b.com", Username: "a", CreatedAt: createdAt, UpdatedAt: createdAt}, nil
		}}
		res, err := NewPlayerUsecase(repo).FindOnePlayerProfile(context.Background(), id.Hex())
		assertErr(t, err, "")
		if res.Id != id.Hex() {
			t.Fatalf("id = %q", res.Id)
		}
		if res.CreatedAt.Location().String() != "Asia/Bangkok" || !res.CreatedAt.Equal(createdAt) {
			t.Fatalf("created at = %v", res.CreatedAt)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &fakePlayerRepo{findProfileFn: func(context.Context, string) (*player.PlayerProfileBson, error) {
			return nil, errors.New("error: player profile not found")
		}}
		_, err := NewPlayerUsecase(repo).FindOnePlayerProfile(context.Background(), id.Hex())
		assertErr(t, err, "player profile not found")
	})
}

func TestAddPlayerMoney(t *testing.T) {
	req := &player.CreatePlayerTransactionReq{PlayerId: "player:abc", Amount: 100}

	t.Run("success", func(t *testing.T) {
		var tx *player.PlayerTransaction
		repo := &fakePlayerRepo{
			insertTransactionFn: func(_ context.Context, r *player.PlayerTransaction) error {
				tx = r
				return nil
			},
			getSavingAccountFn: func(_ context.Context, id string) (*player.PlayerSavingAccount, error) {
				return &player.PlayerSavingAccount{PlayerId: id, Balance: 100}, nil
			},
		}
		res, err := NewPlayerUsecase(repo).AddPlayerMoney(context.Background(), req)
		assertErr(t, err, "")
		if tx.PlayerId != "player:abc" || tx.Amount != 100 || tx.CreatedAt.IsZero() {
			t.Fatalf("transaction = %+v", tx)
		}
		if res.Balance != 100 || res.PlayerId != "player:abc" {
			t.Fatalf("res = %+v", res)
		}
	})

	t.Run("insert failed", func(t *testing.T) {
		repo := &fakePlayerRepo{
			insertTransactionFn: func(context.Context, *player.PlayerTransaction) error {
				return errors.New("error: insert one player transaction failed")
			},
			getSavingAccountFn: func(context.Context, string) (*player.PlayerSavingAccount, error) {
				t.Fatal("must not read balance when insert failed")
				return nil, nil
			},
		}
		_, err := NewPlayerUsecase(repo).AddPlayerMoney(context.Background(), req)
		assertErr(t, err, "insert one player transaction failed")
	})
}

func TestGetPlayerSavingAccount(t *testing.T) {
	repo := &fakePlayerRepo{getSavingAccountFn: func(_ context.Context, id string) (*player.PlayerSavingAccount, error) {
		if id != "player:abc" {
			return nil, errors.New("error: not found")
		}
		return &player.PlayerSavingAccount{PlayerId: id, Balance: 50}, nil
	}}
	uc := NewPlayerUsecase(repo)

	res, err := uc.GetPlayerSavingAccount(context.Background(), "player:abc")
	assertErr(t, err, "")
	if res.Balance != 50 {
		t.Fatalf("balance = %v", res.Balance)
	}

	_, err = uc.GetPlayerSavingAccount(context.Background(), "player:none")
	assertErr(t, err, "not found")
}

func TestFindOnePlayerCredential(t *testing.T) {
	tests := []struct {
		name         string
		password     string
		findErr      error
		wantRoleCode int32
		wantErr      string
	}{
		{name: "success sums role codes", password: "123456", wantRoleCode: 1},
		{name: "wrong password", password: "wrong", wantErr: "password is invalid"},
		{name: "player not found", password: "123456", findErr: errors.New("error: email is invalid"), wantErr: "email is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPlayer(t, "123456",
				player.PlayerRole{RoleTitle: "player", RoleCode: 0},
				player.PlayerRole{RoleTitle: "admin", RoleCode: 1},
			)
			repo := &fakePlayerRepo{findCredentialFn: func(_ context.Context, email string) (*player.Player, error) {
				if email != "test@inwza.com" {
					t.Errorf("email = %q", email)
				}
				if tt.findErr != nil {
					return nil, tt.findErr
				}
				return p, nil
			}}

			res, err := NewPlayerUsecase(repo).FindOnePlayerCredential(context.Background(), "test@inwza.com", tt.password)
			assertErr(t, err, tt.wantErr)
			if tt.wantErr != "" {
				return
			}

			if res.Id != p.Id.Hex() || res.Email != p.Email || res.Username != p.Username {
				t.Fatalf("res = %+v", res)
			}
			if res.RoleCode != tt.wantRoleCode {
				t.Fatalf("role code = %d, want %d", res.RoleCode, tt.wantRoleCode)
			}
			parsed, err := time.Parse(time.RFC3339Nano, res.CreatedAt)
			if err != nil || !parsed.Equal(p.CreatedAt) {
				t.Fatalf("created at = %q (%v)", res.CreatedAt, err)
			}
		})
	}
}

func TestFindOnePlayerProfileToRefresh(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		p := testPlayer(t, "123456", player.PlayerRole{RoleTitle: "player", RoleCode: 0})
		repo := &fakePlayerRepo{findProfileRefreshFn: func(_ context.Context, id string) (*player.Player, error) {
			if id != p.Id.Hex() {
				t.Errorf("player id = %q", id)
			}
			return p, nil
		}}
		res, err := NewPlayerUsecase(repo).FindOnePlayerProfileToRefresh(context.Background(), p.Id.Hex())
		assertErr(t, err, "")
		if res.Id != p.Id.Hex() || res.RoleCode != 0 {
			t.Fatalf("res = %+v", res)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &fakePlayerRepo{findProfileRefreshFn: func(context.Context, string) (*player.Player, error) {
			return nil, errors.New("error: player profile not found")
		}}
		_, err := NewPlayerUsecase(repo).FindOnePlayerProfileToRefresh(context.Background(), "abc")
		assertErr(t, err, "player profile not found")
	})
}
