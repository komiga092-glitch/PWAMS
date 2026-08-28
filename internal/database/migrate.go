package database

import (
	"fmt"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	// One-time data repair so the composite unique index on
	// (loan_id, installment_number) can be created by AutoMigrate even
	// when historical duplicates exist. Kept-before logic removes every
	// later duplicate deterministically (created_at, then id).
	if db.Migrator().HasTable("loan_repayments") {
		dedup := `DELETE FROM loan_repayments a
			USING loan_repayments b
			WHERE a.loan_id = b.loan_id
			  AND a.installment_number = b.installment_number
			  AND (a.created_at > b.created_at
			       OR (a.created_at = b.created_at AND a.id > b.id))`
		if err := db.Exec(dedup).Error; err != nil {
			return fmt.Errorf("loan repayment deduplication failed: %w", err)
		}
	}

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
		&models.Message{},
		&models.Loan{},
		&models.LoanRepayment{},
		&models.RevenueRecord{},
		&models.SyncIdempotencyRecord{},
	); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	return nil
}
