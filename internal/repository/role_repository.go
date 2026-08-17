package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrRoleNotFound = errors.New("role not found")

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) FindByName(name string) (*models.Role, error) {
	var role models.Role

	normalizedName := strings.TrimSpace(name)

	err := r.db.
		Where("LOWER(name) = LOWER(?)", normalizedName).
		First(&role).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoleNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find role: %w", err)
	}

	return &role, nil
}
