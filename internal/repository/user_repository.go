package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) FindByLogin(login string) (*models.User, error) {
	var user models.User

	normalizedLogin := strings.ToLower(strings.TrimSpace(login))

	err := r.db.
		Preload("Role").
		Where(
			"LOWER(username) = ? OR LOWER(email) = ?",
			normalizedLogin,
			normalizedLogin,
		).
		First(&user).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find user by login: %w", err)
	}

	return &user, nil

}

func (r *UserRepository) UpdateLastLogin(userID uuid.UUID) error {
	now := time.Now().UTC()

	if err := r.db.
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("last_login_at", now).
		Error; err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

func (r *UserRepository) ExistsByUsernameOrEmail(
	username string,
	email string,
) (bool, error) {
	var count int64

	err := r.db.
		Model(&models.User{}).
		Where(
			"LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)",
			username,
			email,
		).
		Count(&count).
		Error

	if err != nil {
		return false, fmt.Errorf("failed to check existing user: %w", err)
	}

	return count > 0, nil
}

func (r *UserRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}
