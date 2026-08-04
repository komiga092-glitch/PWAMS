package services

import (
	"errors"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

var (
	ErrInvalidCredentials = errors.New("invalid username/email or password")
	ErrUserDisabled       = errors.New("user account is disabled")
	ErrUserLocked         = errors.New("user account is locked")
)

type AuthService struct {
	userRepo *repository.UserRepository
}

func NewAuthService(userRepo *repository.UserRepository) *AuthService {
	return &AuthService{
		userRepo: userRepo,
	}
}

func (s *AuthService) Login(login string, password string) (*models.User, error) {

	user, err := s.userRepo.FindByLogin(login)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.Status == models.UserStatusDisabled {
		return nil, ErrUserDisabled
	}

	if user.Status == models.UserStatusLocked {
		return nil, ErrUserLocked
	}

	err = utils.CheckPassword(user.PasswordHash, password)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}
