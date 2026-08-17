package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/utils"
	"gorm.io/gorm"
)

func SeedSuperAdmin(db *gorm.DB, cfg *config.Config) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var role models.Role

		if err := tx.
			Where("name = ?", models.RoleSuperAdmin).
			First(&role).Error; err != nil {
			return fmt.Errorf("super admin role not found: %w", err)
		}

		email := strings.ToLower(strings.TrimSpace(cfg.SuperAdminEmail))
		username := strings.TrimSpace(cfg.SuperAdminUsername)

		var existingUser models.User

		err := tx.
			Where("email = ? OR username = ?", email, username).
			First(&existingUser).Error

		if err == nil {
			// Account ஏற்கனவே இருப்பதால் duplicate create செய்ய வேண்டாம்.
			return nil
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to check existing super admin: %w", err)
		}

		passwordHash, err := utils.HashPassword(cfg.SuperAdminPassword)
		if err != nil {
			return fmt.Errorf("failed to hash super admin password: %w", err)
		}

		user := models.User{
			Username:     username,
			Email:        email,
			PasswordHash: passwordHash,
			RoleID:       role.ID,
			Status:       models.UserStatusActive,
		}

		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("failed to create super admin: %w", err)
		}

		return nil
	})
}
