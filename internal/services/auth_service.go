package services

import (
	"errors"
	"fmt"

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
func (s *AuthService) Login(
	login string,
	password string,
) (*models.User, error) {
	user, err := s.userRepo.FindByLogin(login)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("authentication service error: %w", err)
	}

	switch user.Status {
	case models.UserStatusDisabled:
		return nil, ErrUserDisabled

	case models.UserStatusLocked:
		return nil, ErrUserLocked

	case models.UserStatusPending:
		return nil, errors.New("user account is pending approval")

	case models.UserStatusActive:
		// Continue login.

	default:
		return nil, errors.New("user account status is invalid")
	}

	if err := utils.CheckPassword(
		user.PasswordHash,
		password,
	); err != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}
