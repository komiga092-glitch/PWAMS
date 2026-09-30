package database

import (
	"fmt"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

// Migrate is the DEVELOPMENT schema-synchronisation path (GORM AutoMigrate).
//
// PHASE 4D decision: AutoMigrate remains available for development (and for
// the integration tests that call this function) but is never used in
// production startup. cmd/server/main.go calls database.VerifyUpToDate
// (tracked migrations, zero schema writes) when APP_ENV=production instead.
//
// PHASE 4E: the previous "deduplicate loan_repayments on every startup"
// DELETE was removed from this path. Destructive data repair is available
// only through the explicit `go run ./cmd/migrate repair-loan-repayments`
// command (see repair.go / migration 000004). Startup fails closed instead
// when duplicate repayments would prevent the composite unique index.
func Migrate(db *gorm.DB) error {
	// Read-only pre-check: counts duplicates, never modifies data.
	if err := EnsureLoanRepaymentsRepairable(db); err != nil {
		return err
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
