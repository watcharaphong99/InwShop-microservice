package middlewareusecase

import (
	"errors"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/middlerware/middlewareRepository"
	"github.com/watcharaphong99/InwzaShop/pkg/jwtauth"
	"github.com/watcharaphong99/InwzaShop/pkg/rbac"
)

type (
	MiddlewareUsecaseService interface {
		JwtAuthorization(c echo.Context, cfg *config.Config, accessToken string) (echo.Context, error)
		RbacAuthorization(c echo.Context, cfg *config.Config, expected []int) (echo.Context, error)
		PlayerIdParamValidation(c echo.Context) (echo.Context, error)
	}

	middlewareUsecase struct {
		middlewareRepository middlewareRepository.MiddlewareRepositoryService
	}
)

func NewMiddlewareUsecase(middlewareRepository middlewareRepository.MiddlewareRepositoryService) MiddlewareUsecaseService {
	return &middlewareUsecase{middlewareRepository}
}

func (u *middlewareUsecase) JwtAuthorization(c echo.Context, cfg *config.Config, accessToken string) (echo.Context, error) {
	ctx := c.Request().Context()
	claims, err := jwtauth.ParseToken(cfg.Jwt.AccessSecretKey, accessToken)
	if err != nil {
		return nil, err
	}

	if err := u.middlewareRepository.AccessTokenSearch(ctx, cfg.Grpc.AuthUrl, accessToken); err != nil {
		return nil, err
	}

	c.Set("player_id", claims.PlayerId)
	c.Set("role_code", claims.RoleCode)

	return c, nil

}

func (u *middlewareUsecase) RbacAuthorization(c echo.Context, cfg *config.Config, expected []int) (echo.Context, error) {
	ctx := c.Request().Context()

	playerRoleCode, ok := c.Get("role_code").(int)
	if !ok {
		log.Println("Error: role_code not found")
		return nil, errors.New("error: role_code is required")
	}

	rolesCount, err := u.middlewareRepository.RolesCount(ctx, cfg.Grpc.AuthUrl)
	if err != nil {
		return nil, err
	}

	if int(rolesCount) != len(expected) {
		log.Printf("Error: expected roles length is invalid, roles_count: %d, expected_len: %d", rolesCount, len(expected))
		return nil, errors.New("error: expected roles length is invalid")
	}

	for i := 0; i < len(expected); i++ {
		if expected[i] != 0 && expected[i] != 1 {
			log.Printf("Error: expected role must be 0 or 1, index: %d, value: %d", i, expected[i])
			return nil, errors.New("error: expected roles is invalid")
		}
	}

	playerRoleBinary := rbac.IntToBinary(playerRoleCode, int(rolesCount))

	for i := 0; i < int(rolesCount); i++ {
		if expected[i] == 1 && playerRoleBinary[i] == 1 {
			return c, nil
		}
	}

	return nil, errors.New("error: permission denied")
}

func (u *middlewareUsecase) PlayerIdParamValidation(c echo.Context) (echo.Context, error) {
	playerIdReq := c.Param("player_id")
	playerIdToken, ok := c.Get("player_id").(string)
	if !ok {
		log.Println("Error: player_id not found")
		return nil, errors.New("error: player_id is required")
	}

	if playerIdToken == "" {
		log.Println("Error: player_id not found")
		return nil, errors.New("error: player_id is required")
	}

	if playerIdToken != playerIdReq {
		log.Printf("Error: player_id not match,player_id_req: %s, player_id_token: %s", playerIdReq, playerIdToken)
		return nil, errors.New("error: player_id not match")
	}

	return c, nil
}
