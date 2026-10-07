package playerUsecase

import (
	"context"
	"errors"
	"log"
	"math"
	"time"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/payment"
	"github.com/watcharaphong99/InwzaShop/modules/player"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/modules/player/playerRepository"

	"github.com/watcharaphong99/InwzaShop/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

type (
	PlayerUsecaseService interface {
		GetOffset(pctx context.Context) (int64, error)
		UpsertOffset(pctx context.Context, offset int64) error
		CreatePlayer(pctx context.Context, req *player.CreatePlayerReq) (*player.PlayerProfile, error)
		FindOnePlayerProfile(pctx context.Context, playerId string) (*player.PlayerProfile, error)
		AddPlayerMoney(pctx context.Context, req *player.CreatePlayerTransactionReq) (*player.PlayerSavingAccount, error)
		GetPlayerSavingAccount(pctx context.Context, playerId string) (*player.PlayerSavingAccount, error)
		FindOnePlayerCredential(pctx context.Context, email, password string) (*playerPb.PlayerProfile, error)
		FindOnePlayerProfileToRefresh(pctx context.Context, playerId string) (*playerPb.PlayerProfile, error)
		RollbackPlayerTransaction(pctx context.Context, req *player.RollbackPlayerTransactionReq)
		DockedPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq)
		AddPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq)
	}

	playerUsecase struct {
		playerRepository playerRepository.PlayerRepositoryService
	}
)

func NewPlayerUsecase(playerRepository playerRepository.PlayerRepositoryService) PlayerUsecaseService {
	return &playerUsecase{playerRepository: playerRepository}
}

func (u *playerUsecase) GetOffset(pctx context.Context) (int64, error) {
	return u.playerRepository.GetOffset(pctx)
}

func (u *playerUsecase) UpsertOffset(pctx context.Context, offset int64) error {
	return u.playerRepository.UpsertOffset(pctx, offset)
}

func (u *playerUsecase) CreatePlayer(pctx context.Context, req *player.CreatePlayerReq) (*player.PlayerProfile, error) {
	if !u.playerRepository.IsUniquePlayer(pctx, req.Email, req.Username) {
		return nil, errors.New("error: email or username already exist")
	}

	//Hashing password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("error: failed to hash password")
	}

	//Insert one player
	playerId, err := u.playerRepository.InsertOnePlayer(pctx, &player.Player{
		Email:     req.Email,
		Password:  string(hashedPassword),
		Username:  req.Username,
		CreatedAt: utils.LocalTime(),
		UpdatedAt: utils.LocalTime(),
		PlayerRoles: []player.PlayerRole{
			{
				RoleTitle: "player",
				RoleCode:  0,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return u.FindOnePlayerProfile(pctx, playerId.Hex())

}

func (u *playerUsecase) FindOnePlayerProfile(pctx context.Context, playerId string) (*player.PlayerProfile, error) {
	result, err := u.playerRepository.FindOnePlayerProfine(pctx, playerId)
	if err != nil {
		return nil, err
	}

	loc, _ := time.LoadLocation("Asia/Bangkok")

	return &player.PlayerProfile{
		Id:        result.Id.Hex(),
		Email:     result.Email,
		Username:  result.Username,
		CreatedAt: result.CreatedAt.In(loc),
		UpdatedAt: result.UpdatedAt.In(loc),
	}, nil

}

func (u *playerUsecase) AddPlayerMoney(pctx context.Context, req *player.CreatePlayerTransactionReq) (*player.PlayerSavingAccount, error) {
	//Inseart one player transaction

	log.Print("playerId", req.PlayerId)

	if _, err := u.playerRepository.InsertOnePlayerTranscation(pctx, &player.PlayerTransaction{
		PlayerId:  req.PlayerId,
		Amount:    req.Amount,
		EventId:   req.EventId,
		CreatedAt: utils.LocalTime(),
	}); err != nil {
		return nil, err
	}

	return u.playerRepository.GetPlayerSavingAccount(pctx, req.PlayerId)
}

func (u *playerUsecase) GetPlayerSavingAccount(pctx context.Context, playerId string) (*player.PlayerSavingAccount, error) {
	return u.playerRepository.GetPlayerSavingAccount(pctx, playerId)
}

func (u *playerUsecase) FindOnePlayerCredential(pctx context.Context, email, password string) (*playerPb.PlayerProfile, error) {
	result, err := u.playerRepository.FindOnePlayerCredential(pctx, email)

	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(password)); err != nil {
		log.Printf("Error: FindOnePlayerCredential: %s", err.Error())
		return nil, errors.New("error: password is invalid")
	}

	roleCode := 0
	for _, v := range result.PlayerRoles {
		roleCode += v.RoleCode
	}

	loc, _ := time.LoadLocation("Asia/Bangkok")

	return &playerPb.PlayerProfile{
		Id:        result.Id.Hex(),
		Email:     result.Email,
		Username:  result.Username,
		RoleCode:  int32(roleCode),
		CreatedAt: result.CreatedAt.In(loc).Format(time.RFC3339Nano),
		UpdatedAt: result.UpdatedAt.In(loc).Format(time.RFC3339Nano),
	}, nil

}

func (u *playerUsecase) FindOnePlayerProfileToRefresh(pctx context.Context, playerId string) (*playerPb.PlayerProfile, error) {
	result, err := u.playerRepository.FindOnePlayerProfileTokenRefresh(pctx, playerId)
	if err != nil {
		return nil, err
	}

	roleCode := 0
	for _, v := range result.PlayerRoles {
		roleCode += v.RoleCode
	}

	loc, _ := time.LoadLocation("Asia/Bangkok")

	return &playerPb.PlayerProfile{
		Id:        result.Id.Hex(),
		Email:     result.Email,
		Username:  result.Username,
		RoleCode:  int32(roleCode),
		CreatedAt: result.CreatedAt.In(loc).Format(time.RFC3339Nano),
		UpdatedAt: result.UpdatedAt.In(loc).Format(time.RFC3339Nano),
	}, nil
}

func (u *playerUsecase) DockedPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq) {
	//Get saving account ตรวจสอบว่ามีเงินพอที่จะซื้อของมั้ย
	savingAccount, err := u.playerRepository.GetPlayerSavingAccount(pctx, req.PlayerId)
	if err != nil {
		u.playerRepository.DockedPlayerMoneyRes(pctx, cfg, &payment.PaymentTransferRes{
			InventoryId:   "",
			TransactionId: "",
			PlayerId:      req.PlayerId,
			ItemId:        "",
			Amount:        req.Amount,
			Error:         err.Error(),
		})
		return
	}

	// เงินพอมมั้ยที่จะซื้อของ
	if savingAccount.Balance < math.Abs(req.Amount) {
		log.Printf("Error: DockedPlayerMoneyRes failed: %s", "not enough money")
		u.playerRepository.DockedPlayerMoneyRes(pctx, cfg, &payment.PaymentTransferRes{
			InventoryId:   "",
			TransactionId: "",
			PlayerId:      req.PlayerId,
			ItemId:        "",
			Amount:        req.Amount,
			Error:         "error: not enough money",
		})
		return
	}

	transactionId, err := u.playerRepository.InsertOnePlayerTranscation(pctx, &player.PlayerTransaction{
		PlayerId:  req.PlayerId,
		Amount:    req.Amount,
		CreatedAt: utils.LocalTime(),
	})

	if err != nil {
		u.playerRepository.DockedPlayerMoneyRes(pctx, cfg, &payment.PaymentTransferRes{
			InventoryId:   "",
			TransactionId: "",
			PlayerId:      req.PlayerId,
			ItemId:        "",
			Amount:        req.Amount,
			Error:         err.Error(),
		})
		return
	}

	u.playerRepository.DockedPlayerMoneyRes(pctx, cfg, &payment.PaymentTransferRes{
		InventoryId:   "",
		TransactionId: transactionId.Hex(),
		PlayerId:      req.PlayerId,
		ItemId:        "",
		Amount:        req.Amount,
		Error:         "",
	})

}

func (u *playerUsecase) AddPlayerMoneyRes(pctx context.Context, cfg *config.Config, req *player.CreatePlayerTransactionReq) {
	// Insert one player transaction
	transactionId, err := u.playerRepository.InsertOnePlayerTranscation(pctx, &player.PlayerTransaction{
		PlayerId:  req.PlayerId,
		Amount:    req.Amount,
		CreatedAt: utils.LocalTime(),
	})
	if err != nil {
		u.playerRepository.AddPlayerMoneyRes(pctx, cfg, &payment.PaymentTransferRes{
			InventoryId:   "",
			TransactionId: "",
			PlayerId:      req.PlayerId,
			ItemId:        "",
			Amount:        req.Amount,
			Error:         err.Error(),
		})
		return
	}

	u.playerRepository.AddPlayerMoneyRes(pctx, cfg, &payment.PaymentTransferRes{
		InventoryId:   "",
		TransactionId: transactionId.Hex(),
		PlayerId:      req.PlayerId,
		ItemId:        "",
		Amount:        req.Amount,
		Error:         "",
	})
}

func (u *playerUsecase) RollbackPlayerTransaction(pctx context.Context, req *player.RollbackPlayerTransactionReq) {
	u.playerRepository.DeleteOnePlayerTransaction(pctx, req.TransactionId)
}
