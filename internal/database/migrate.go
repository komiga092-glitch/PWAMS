package database

import (
	"fmt"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.Role{},
		&models.User{},
	); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	return nil
}
