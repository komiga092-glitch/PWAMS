package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

const (
	maxFailedLoginAttempts = 3
	lockoutDuration        = 30 * time.Minute
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
	login, password string,
) (*models.User, error) {
	user, err := s.userRepo.FindByLogin(login)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf("authentication service error: %w", err)
	}

	if user.IsLocked() {
		return nil, ErrUserLocked
	}

	switch user.Status {
	case models.UserStatusDisabled:
		return nil, ErrUserDisabled

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
		user.FailedLoginAttempts++
		if user.FailedLoginAttempts >= maxFailedLoginAttempts {
			lockedUntil := time.Now().Add(lockoutDuration)
			user.LockedUntil = &lockedUntil
			user.Status = models.UserStatusLocked
		}
		if persistErr := s.userRepo.UpdateLoginAttempts(user); persistErr != nil {
			log.Printf("unable to persist failed-login counter for %q: %v", login, persistErr)
		}
		return nil, ErrInvalidCredentials
	}

	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	if user.Status == models.UserStatusLocked {
		user.Status = models.UserStatusActive
	}
	if persistErr := s.userRepo.UpdateLoginAttempts(user); persistErr != nil {
		log.Printf("unable to reset failed-login counter for %q: %v", login, persistErr)
	}

	if err := s.userRepo.UpdateLastLogin(user.ID); err != nil {
		return nil, fmt.Errorf("authentication service error: %w", err)
	}

	return user, nil
}
