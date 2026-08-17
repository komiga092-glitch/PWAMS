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
		&models.Session{},
		&models.Person{},
		&models.Student{},
		&models.Donor{},
		&models.Donation{},
		&models.AidRequest{},
		&models.PasswordResetToken{},
		&models.AccountActivationToken{},
		&models.CareProvided{},
		&models.FileUpload{},
		&models.AuditLog{},
		&models.Notification{},
		&models.Loan{},
		&models.LoanRepayment{},
	); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	return nil
}
