package repository

import (
	"errors"
	"fmt"
	"strings"

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
