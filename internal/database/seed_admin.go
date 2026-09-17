package database

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

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

		passwordHash, err := utils.HashPassword(cfg.SuperAdminPassword)
		if err != nil {
			return fmt.Errorf("failed to hash super admin password: %w", err)
		}

		var existingUser models.User

		err = tx.
			Where("email = ? OR username = ?", email, username).
			First(&existingUser).Error

		if err == nil {
			// The configured seed identifiers already belong to an account.
			// Reconcile the credentials only when that account is the Super
			// Admin itself. Never overwrite the credentials of a different
			// (non-super-admin) account that merely shares the configured
			// username or email, and never create a duplicate Super Admin.
			if existingUser.RoleID != role.ID {
				log.Printf(
					"super admin seed skipped: %q / %q is taken by a non-super-admin account",
					username,
					email,
				)
				return nil
			}

			// Reconcile the configured Super Admin credentials with the
			// existing account: preserve its ID, role and unrelated profile
			// fields while refreshing the seed identifiers, the bcrypt
			// password hash, and the authentication state so the configured
			// credentials log in.
			return tx.Model(&existingUser).Updates(map[string]interface{}{
				"username":              username,
				"email":                 email,
				"password_hash":         passwordHash,
				"status":                models.UserStatusActive,
				"failed_login_attempts": 0,
				"locked_until":          nil,
				"updated_at":            time.Now().UTC(),
			}).Error
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to check existing super admin: %w", err)
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
