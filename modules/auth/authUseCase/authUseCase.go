package authUsecase

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/auth"
	authPb "github.com/watcharaphong99/InwzaShop/modules/auth/authPb"
	"github.com/watcharaphong99/InwzaShop/modules/auth/authRepository"
	"github.com/watcharaphong99/InwzaShop/modules/player"
	playerPb "github.com/watcharaphong99/InwzaShop/modules/player/playerPb"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"github.com/watcharaphong99/InwzaShop/pkg/utils"
)

type (
	AuthUsecaseService interface {
		Login(pctx context.Context, cfg *config.Config, req *auth.PlayerLoginReq) (*auth.ProfileIntercepter, error)
		RefreshToken(pctx context.Context, cfg *config.Config, req *auth.RefreshTokenReq) (*auth.ProfileIntercepter, error)
		Logout(pctx context.Context, credentialId string) (int64, error)
		AccessTokenSearch(pctx context.Context, accessToken string) (*authPb.AccessTokenSearchRes, error)
		RolesCount(pctx context.Context) (*authPb.RolesCountRes, error)
	}

	authUsecase struct {
		authRepository authRepository.AuthRepositoryService
	}
)

func NewAuthUseCase(authRepository authRepository.AuthRepositoryService) AuthUsecaseService {
	return &authUsecase{authRepository: authRepository}
}

func (u *authUsecase) Login(pctx context.Context, cfg *config.Config, req *auth.PlayerLoginReq) (*auth.ProfileIntercepter, error) {
	profile, err := u.authRepository.CredentialSearch(pctx, cfg.Grpc.PlayerUrl, &playerPb.CredentialSearchReq{
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		return nil, err
	}

	playerId := formatPlayerId(profile.Id)

	accessToken := jwtauth.NewAccessToken(cfg.Jwt.AccessSecretKey, cfg.Jwt.AccessDuration, &jwtauth.Claims{
		PlayerId: playerId,
		RoleCode: int(profile.RoleCode),
	}).SignToken()

	refreshToken := jwtauth.NewRefreshToken(cfg.Jwt.RefreshSecretKey, cfg.Jwt.RefreshDuration, &jwtauth.Claims{
		PlayerId: playerId,
		RoleCode: int(profile.RoleCode),
	}).SignToken()

	credentialId, err := u.authRepository.InsertOnePlayerCredential(pctx, &auth.Credential{
		PlayerId:     playerId,
		RoleCode:     int(profile.RoleCode),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    utils.LocalTime(),
		UpdatedAt:    utils.LocalTime(),
	})
	if err != nil {
		return nil, err
	}

	credential, err := u.authRepository.FindOnePlayerCredential(pctx, credentialId.Hex())
	if err != nil {
		return nil, err
	}

	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		return nil, err
	}

	return &auth.ProfileIntercepter{
		PlayerProfile: &player.PlayerProfile{
			Id:        playerId,
			Email:     profile.Email,
			Username:  profile.Username,
			CreatedAt: utils.ConvertStringTimeToTime(profile.CreatedAt).In(loc),
			UpdatedAt: utils.ConvertStringTimeToTime(profile.UpdatedAt).In(loc),
		},
		Credential: &auth.CredentialRes{
			Id:           credential.Id.Hex(),
			PlayerId:     credential.PlayerId,
			RoleCode:     credential.RoleCode,
			AccessToken:  credential.AccessToken,
			RefreshToken: credential.RefreshToken,
			CreatedAt:    credential.CreatedAt.In(loc),
			UpdatedAt:    credential.UpdatedAt.In(loc),
		},
	}, nil
}

func (u *authUsecase) RefreshToken(pctx context.Context, cfg *config.Config, req *auth.RefreshTokenReq) (*auth.ProfileIntercepter, error) {
	claims, err := jwtauth.ParseToken(cfg.Jwt.RefreshSecretKey, req.RefreshToken)
	if err != nil {
		log.Printf("Error: RefreshToken: %s", err.Error())
		return nil, err
	}

	if claims.Subject != "refresh-token" {
		return nil, errors.New("error: token is invalid")
	}
	if claims.ExpiresAt == nil {
		return nil, errors.New("error: token is invalid")
	}

	credential, err := u.authRepository.FindOnePlayerCredential(pctx, req.CredentialId)
	if err != nil {
		return nil, err
	}

	if credential.RefreshToken != req.RefreshToken {
		return nil, errors.New("error: refresh token is invalid")
	}

	profile, err := u.authRepository.FindOnePlayerProfileToRefresh(pctx, cfg.Grpc.PlayerUrl, &playerPb.FindOnePlayerProfileToRefreshReq{
		PlayerId: strings.TrimPrefix(claims.PlayerId, "player:"),
	})
	if err != nil {
		return nil, err
	}

	playerId := formatPlayerId(profile.Id)

	accessToken := jwtauth.NewAccessToken(cfg.Jwt.AccessSecretKey, cfg.Jwt.AccessDuration, &jwtauth.Claims{
		PlayerId: playerId,
		RoleCode: int(profile.RoleCode),
	}).SignToken()

	refreshToken := jwtauth.ReloadToken(cfg.Jwt.RefreshSecretKey, claims.ExpiresAt.Unix(), &jwtauth.Claims{
		PlayerId: playerId,
		RoleCode: int(profile.RoleCode),
	})

	if err := u.authRepository.UpdateOnePlayerCredential(pctx, req.CredentialId, &auth.UpdateRefreshTokenReq{
		PlayerId:     playerId,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UpdatedAt:    utils.LocalTime(),
	}); err != nil {
		return nil, err
	}

	credential, err = u.authRepository.FindOnePlayerCredential(pctx, req.CredentialId)
	if err != nil {
		return nil, err
	}

	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		return nil, err
	}

	return &auth.ProfileIntercepter{
		PlayerProfile: &player.PlayerProfile{
			Id:        playerId,
			Email:     profile.Email,
			Username:  profile.Username,
			CreatedAt: utils.ConvertStringTimeToTime(profile.CreatedAt).In(loc),
			UpdatedAt: utils.ConvertStringTimeToTime(profile.UpdatedAt).In(loc),
		},
		Credential: &auth.CredentialRes{
			Id:           credential.Id.Hex(),
			PlayerId:     credential.PlayerId,
			RoleCode:     credential.RoleCode,
			AccessToken:  credential.AccessToken,
			RefreshToken: credential.RefreshToken,
			CreatedAt:    credential.CreatedAt.In(loc),
			UpdatedAt:    credential.UpdatedAt.In(loc),
		},
	}, nil
}

func formatPlayerId(id string) string {
	return "player:" + strings.TrimPrefix(id, "player:")
}

func (u *authUsecase) Logout(pctx context.Context, credentialId string) (int64, error) {
	return u.authRepository.DeleteOnePlayerCredential(pctx, credentialId)
}

func (u *authUsecase) AccessTokenSearch(pctx context.Context, accessToken string) (*authPb.AccessTokenSearchRes, error) {
	credential, err := u.authRepository.FindOneAccessToken(pctx, accessToken)
	if err != nil {
		return &authPb.AccessTokenSearchRes{
			IsValid: false,
		}, err
	}

	if credential == nil {
		return &authPb.AccessTokenSearchRes{
			IsValid: false,
		}, errors.New("error: access token is invalid")
	}

	return &authPb.AccessTokenSearchRes{
		IsValid: true,
	}, nil
}

func (u *authUsecase) RolesCount(pctx context.Context) (*authPb.RolesCountRes, error) {
	result, err := u.authRepository.RolesCount(pctx)
	if err != nil {
		return nil, err
	}

	return &authPb.RolesCountRes{
		Count: result,
	}, nil
}
