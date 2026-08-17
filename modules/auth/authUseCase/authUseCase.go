package authUsecase

import "github.com/watcharaphong99/InwzaShop/modules/auth/authRepository"

type (
	AuthUsecaseService interface{}

	authUsecase struct {
		authRepository authRepository.AuthRepositoryService
	}
)

func NewAuthUseCase(authRepository authRepository.AuthRepositoryService) AuthUsecaseService {
	return &authUsecase{authRepository: authRepository}
}
