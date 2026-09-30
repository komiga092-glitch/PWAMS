package services_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// Regression tests for the Phase-1 audit-log dependency fix.
//
// The original production bug: handlers were constructed WITHOUT an
// AuditLogService (variadic constructor parameter), performed the business
// write first and then called a method on a nil *AuditLogService. The call
// panicked AFTER the row was committed, the client received HTTP 500 and a
// retry created duplicate records.
//
// These tests verify the new fail-closed contract, which requires no live
// database: a nil or unconfigured audit service must NEVER panic and must
// never allow a business write that cannot be paired with its audit row.

// A nil *AuditLogService must return ErrAuditLogUnavailable from Create
// instead of panicking with a nil pointer dereference.
func TestNilAuditServiceCreateFailsClosed(t *testing.T) {
	var svc *services.AuditLogService

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Create on nil audit service panicked: %v", r)
		}
	}()

	err := svc.Create("00000000-0000-0000-0000-000000000001", "CREATE", "persons", "00000000-0000-0000-0000-000000000002", "x")
	if !errors.Is(err, services.ErrAuditLogUnavailable) {
		t.Fatalf("Create on nil audit service err = %v, want ErrAuditLogUnavailable", err)
	}
}

// Transaction on a nil service must fail closed WITHOUT running the callback:
// the business write inside it is never attempted, so no record can be
// committed without its audit event.
func TestNilAuditServiceTransactionFailsClosed(t *testing.T) {
	var svc *services.AuditLogService

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Transaction on nil audit service panicked: %v", r)
		}
	}()

	businessWriteRan := false
	err := svc.Transaction(func(_ *gorm.DB) error {
		businessWriteRan = true
		return nil
	})
	if !errors.Is(err, services.ErrAuditLogUnavailable) {
		t.Fatalf("Transaction on nil audit service err = %v, want ErrAuditLogUnavailable", err)
	}
	if businessWriteRan {
		t.Fatal("business write callback ran although audit logging is unavailable; the record would be committed without its audit event")
	}
}

// Audit (the mandatory in-transaction write) must also be nil-safe.
func TestNilAuditServiceAuditFailsClosed(t *testing.T) {
	var svc *services.AuditLogService

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Audit on nil audit service panicked: %v", r)
		}
	}()

	err := svc.Audit(nil, "00000000-0000-0000-0000-000000000001", "CREATE", "persons", "00000000-0000-0000-0000-000000000002", "x")
	if !errors.Is(err, services.ErrAuditLogWriteFailed) {
		t.Fatalf("Audit on nil audit service err = %v, want ErrAuditLogWriteFailed", err)
	}
}

// Read paths must be nil-safe too (they are called by the audit log admin UI).
func TestNilAuditServiceReadsFailClosed(t *testing.T) {
	var svc *services.AuditLogService

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil audit service read panicked: %v", r)
		}
	}()

	if _, err := svc.GetByID("00000000-0000-0000-0000-000000000001"); !errors.Is(err, services.ErrAuditLogUnavailable) {
		t.Fatalf("GetByID err = %v, want ErrAuditLogUnavailable", err)
	}
	if _, _, _, _, err := svc.List(models.AuditLogListQuery{}); !errors.Is(err, services.ErrAuditLogUnavailable) {
		t.Fatalf("List err = %v, want ErrAuditLogUnavailable", err)
	}
	if err := svc.Delete("00000000-0000-0000-0000-000000000001"); !errors.Is(err, services.ErrAuditLogUnavailable) {
		t.Fatalf("Delete err = %v, want ErrAuditLogUnavailable", err)
	}
}

// Transactional atomicity (TASK 4): if the mandatory audit write fails, the
// business write inside the same transaction must be rolled back. This is the
// regression test for "business record committed, audit entry missing /
// client saw 500 and retried, creating a duplicate".
func TestAuditFailureRollsBackBusinessWrite(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	roles := loadSeedRoles(t, db)

	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))
	personSvc := services.NewPersonService(repository.NewPersonRepository(db))

	actor := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "rbactor")
	suffix := strings.ToUpper(uuid.New().String()[:8])
	nic := "ROLLBACK-" + suffix
	defer func() {
		_ = db.Unscoped().Delete(&models.Person{}, "nic_passport = ?", nic).Error
	}()

	// The audit write uses a malformed user ID, which Audit rejects
	// (ErrInvalidAuditLogUserID wrapped in ErrAuditLogWriteFailed). The
	// person write happens first inside the same transaction, so the
	// rollback must undo it.
	err := auditSvc.Transaction(func(tx *gorm.DB) error {
		if _, createErr := personSvc.WithTx(tx).CreatePerson(models.CreatePersonRequest{
			FullName:    "Rollback Person " + suffix,
			NICPassport: nic,
			Gender:      "Female",
		}, actor.ID); createErr != nil {
			return createErr
		}

		return auditSvc.Audit(tx, "not-a-uuid", "CREATE", "persons", "", "Person created successfully")
	})
	if !errors.Is(err, services.ErrAuditLogWriteFailed) {
		t.Fatalf("expected ErrAuditLogWriteFailed, got %v", err)
	}

	var count int64
	if err := db.Model(&models.Person{}).Where("nic_passport = ?", nic).Count(&count).Error; err != nil {
		t.Fatalf("person count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("person row survived audit failure: %d rows; transaction must roll back the business write", count)
	}
}

// The happy path must still commit BOTH rows atomically.
func TestAuditSuccessCommitsBothRows(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	roles := loadSeedRoles(t, db)

	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))
	personSvc := services.NewPersonService(repository.NewPersonRepository(db))

	actor := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "cmactor")
	suffix := strings.ToUpper(uuid.New().String()[:8])
	nic := "COMMIT-" + suffix
	actorID := actor.ID
	defer func() {
		_ = db.Unscoped().Delete(&models.Person{}, "nic_passport = ?", nic).Error
		_ = db.Unscoped().Delete(&models.AuditLog{}, "entity = ? AND user_id = ?", "persons", actorID).Error
	}()

	var personID string
	err := auditSvc.Transaction(func(tx *gorm.DB) error {
		created, createErr := personSvc.WithTx(tx).CreatePerson(models.CreatePersonRequest{
			FullName:    "Commit Person " + suffix,
			NICPassport: nic,
			Gender:      "Male",
		}, actorID)
		if createErr != nil {
			return createErr
		}
		personID = created.ID.String()
		return auditSvc.Audit(tx, actorID.String(), "CREATE", "persons", personID, "Person created successfully")
	})
	if err != nil {
		t.Fatalf("transactional create failed: %v", err)
	}

	var persons int64
	if err := db.Model(&models.Person{}).Where("nic_passport = ?", nic).Count(&persons).Error; err != nil {
		t.Fatalf("person count failed: %v", err)
	}
	if persons != 1 {
		t.Fatalf("person rows = %d, want exactly 1", persons)
	}
	if n := countAuditRows(t, db, "persons", "CREATE", actorID); n != 1 {
		t.Fatalf("audit rows = %d, want exactly 1", n)
	}
}
