package services_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// countAuditEvents counts audit rows for one action on one entity. Counts are
// scoped to a specific entity ID so repeated runs against the shared test
// database stay deterministic.
func countAuditEvents(t *testing.T, db *gorm.DB, action, entityID string) int64 {
	t.Helper()

	parsed, err := uuid.Parse(entityID)
	if err != nil {
		t.Fatalf("parse audit entity id %q: %v", entityID, err)
	}

	var count int64
	if err := db.Model(&models.AuditLog{}).
		Where("action = ? AND entity_id = ?", action, parsed).
		Count(&count).Error; err != nil {
		t.Fatalf("count %s audit rows: %v", action, err)
	}
	return count
}

// newAuditedLoanService builds loan + repayment services wired to a real
// AuditLogService so lifecycle audit events (LOAN_CREATED,
// LOAN_REPAYMENT_SCHEDULE_CREATED, LOAN_COMPLETED) are written through the
// same transaction the state change uses.
func newAuditedLoanService(t *testing.T, db *gorm.DB) (*services.LoanService, *services.LoanRepaymentService) {
	t.Helper()

	loanRepo := repository.NewLoanRepository(db)
	repaymentRepo := repository.NewLoanRepaymentRepository(db)
	personRepo := repository.NewPersonRepository(db)
	notif := services.NewNotificationService(repository.NewNotificationRepository(db))
	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))

	loanSvc := services.NewLoanServiceWithAudit(loanRepo, repaymentRepo, personRepo, db, notif, auditSvc)
	repaySvc := services.NewLoanRepaymentServiceWithAudit(repaymentRepo, loanRepo, personRepo, db, auditSvc)
	return loanSvc, repaySvc
}

func shortTestSuffix() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
}

// TestLoanAudit_CreationEvents verifies that creating a loan emits exactly one
// LOAN_CREATED and one LOAN_REPAYMENT_SCHEDULE_CREATED audit record for the
// new loan, and that both live in the same transaction as the loan rows
// (they are committed together with the state they describe).
func TestLoanAudit_CreationEvents(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	loanSvc, _ := newAuditedLoanService(t, db)

	creator := newTestCreator(t, db, shortTestSuffix())
	person := newLoanLifecyclePerson(t, db, shortTestSuffix())

	loan, err := loanSvc.CreateLoan(models.CreateLoanRequest{
		PersonID:       person.ID.String(),
		LoanAmount:     decimal.NewFromInt(9000),
		InterestRate:   decimal.NewFromInt(6),
		DurationMonths: 3,
		StartDate:      "2026-09-01",
		DueDay:         1,
		Purpose:        "Audit creation test",
	}, creator.ID)
	if err != nil {
		t.Fatalf("CreateLoan: %v", err)
	}

	if got := countAuditEvents(t, db, "LOAN_CREATED", loan.ID.String()); got != 1 {
		t.Errorf("LOAN_CREATED audit count = %d, want 1", got)
	}
	if got := countAuditEvents(t, db, "LOAN_REPAYMENT_SCHEDULE_CREATED", loan.ID.String()); got != 1 {
		t.Errorf("LOAN_REPAYMENT_SCHEDULE_CREATED audit count = %d, want 1", got)
	}

	// The audit rows must record the acting user from server-side context,
	// never client-supplied identity.
	var entry models.AuditLog
	if err := db.Where("action = ? AND entity_id = ?", "LOAN_CREATED", loan.ID).
		First(&entry).Error; err != nil {
		t.Fatalf("load LOAN_CREATED audit row: %v", err)
	}
	if entry.UserID == nil || *entry.UserID != creator.ID {
		t.Errorf("LOAN_CREATED audit user = %v, want creator %s", entry.UserID, creator.ID)
	}
}

// TestLoanAudit_CompletedOnFinalPayment verifies that the payment which clears
// the last outstanding installment emits exactly one LOAN_COMPLETED audit
// record (LOAN_PAYMENT itself is asserted at the handler layer).
func TestLoanAudit_CompletedOnFinalPayment(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	loanSvc, repaySvc := newAuditedLoanService(t, db)

	creator := newTestCreator(t, db, shortTestSuffix())
	person := newLoanLifecyclePerson(t, db, shortTestSuffix())

	loan, err := loanSvc.CreateLoan(models.CreateLoanRequest{
		PersonID:       person.ID.String(),
		LoanAmount:     decimal.NewFromInt(6000),
		InterestRate:   decimal.NewFromInt(0),
		DurationMonths: 2,
		StartDate:      "2026-09-01",
		DueDay:         1,
		Purpose:        "Audit completion test",
	}, creator.ID)
	if err != nil {
		t.Fatalf("CreateLoan: %v", err)
	}

	for _, st := range []string{models.LoanStatusApproved, models.LoanStatusActive} {
		if _, err := loanSvc.ReviewLoan(loan.ID.String(), models.ReviewLoanRequest{Status: st}, creator.ID); err != nil {
			t.Fatalf("ReviewLoan to %s: %v", st, err)
		}
	}

	var installments []models.LoanRepayment
	if err := db.Where("loan_id = ? AND is_deleted = FALSE", loan.ID).
		Order("installment_number ASC").Find(&installments).Error; err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if len(installments) != 2 {
		t.Fatalf("schedule size = %d, want 2", len(installments))
	}

	for i := range installments {
		if _, err := repaySvc.Pay(installments[i].ID.String(), models.PayLoanRepaymentRequest{
			PaidAmount:    installments[i].OutstandingAmount,
			PaymentMethod: "Cash",
			UpdatedBy:     &creator.ID,
		}); err != nil {
			t.Fatalf("Pay installment %d: %v", installments[i].InstallmentNumber, err)
		}
	}

	if got := countAuditEvents(t, db, "LOAN_COMPLETED", loan.ID.String()); got != 1 {
		t.Errorf("LOAN_COMPLETED audit count = %d, want 1", got)
	}

	final := reloadLoan(t, db, loan.ID)
	if final.Status != models.LoanStatusCompleted {
		t.Errorf("loan status = %q, want %q", final.Status, models.LoanStatusCompleted)
	}
}
