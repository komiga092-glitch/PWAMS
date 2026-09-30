package database

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// PHASE 4F — tracked migration runner behaviour, verified from a clean
// baseline inside the dedicated migration test schema.

func TestMigrationStatusAndVerifyAreReadOnly(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	statuses, err := Status(db)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(statuses) != 9 {
		t.Fatalf("expected 9 known migrations, got %d", len(statuses))
	}
	for _, state := range statuses {
		if state.Applied {
			t.Fatalf("migration %06d unexpectedly applied on a clean baseline", state.Version)
		}
		if !state.ChecksumMatch {
			t.Fatalf("migration %06d checksum mismatch on a clean baseline", state.Version)
		}
	}

	if HasMigrationTable(db) {
		t.Fatal("Status() must not create the schema_migrations tracking table")
	}

	err = VerifyUpToDate(db)
	if err == nil {
		t.Fatal("VerifyUpToDate must fail while migrations are pending")
	}
	if !strings.Contains(err.Error(), "pending migrations") || !strings.Contains(err.Error(), "000009") {
		t.Fatalf("pending error should list pending versions, got: %v", err)
	}
	if HasMigrationTable(db) {
		t.Fatal("VerifyUpToDate must not create the tracking table")
	}
}

func TestMigrationsUpFromCleanBaseline(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUp(db); err != nil {
		t.Fatalf("up: %v", err)
	}

	applied, err := AppliedMigrationsReadOnly(db)
	if err != nil {
		t.Fatalf("applied: %v", err)
	}
	if len(applied) != 9 {
		t.Fatalf("expected 9 applied migrations, got %d", len(applied))
	}

	files, err := LoadMigrationFiles()
	if err != nil {
		t.Fatalf("files: %v", err)
	}
	for _, file := range files {
		record, ok := applied[file.Version]
		if !ok {
			t.Fatalf("migration %06d not tracked", file.Version)
		}
		if record.Checksum != file.Checksum {
			t.Fatalf("checksum mismatch for %06d", file.Version)
		}
	}

	// Ordering: versions must be recorded in ascending application order.
	var order []int64
	if err := db.Raw(`SELECT version FROM schema_migrations ORDER BY applied_at, version`).Scan(&order).Error; err != nil {
		t.Fatalf("order: %v", err)
	}
	for i, version := range order {
		if version != int64(i+1) {
			t.Fatalf("migration order broken: %v", order)
		}
	}

	// Expected schema (canonical-shape spot checks).
	mustHaveColumn(t, db, "organizations", "code")
	mustHaveColumn(t, db, "users", "organization_id")
	mustHaveColumn(t, db, "admin_deletion_requests", "target_user_id")
	mustHaveColumn(t, db, "loan_repayments", "installment_number")
	mustHaveColumn(t, db, "loan_repayments", "due_date")
	mustHaveColumn(t, db, "loan_repayments", "paid_amount")
	mustNotHaveColumn(t, db, "audit_logs", "ip_address") // 000002 — no-IP policy

	if !hasUniqueIndex(t, db, "loan_repayments", "uk_loan_repayment_installment") {
		t.Fatal("uk_loan_repayment_installment missing after up")
	}
	if !hasTrigger(t, db, "users", "trg_users_one_active_manager") {
		t.Fatal("trg_users_one_active_manager missing after up")
	}

	if err := VerifyUpToDate(db); err != nil {
		t.Fatalf("verify after up: %v", err)
	}
}

func TestMigrationsUpIsIdempotent(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUp(db); err != nil {
		t.Fatalf("first up: %v", err)
	}
	first, err := AppliedMigrationsReadOnly(db)
	if err != nil {
		t.Fatalf("applied: %v", err)
	}

	if err := MigrateUp(db); err != nil {
		t.Fatalf("repeat up: %v", err)
	}
	second, err := AppliedMigrationsReadOnly(db)
	if err != nil {
		t.Fatalf("applied: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("repeat up changed the applied set: %d -> %d", len(first), len(second))
	}
	for version, record := range first {
		other, ok := second[version]
		if !ok || other.Checksum != record.Checksum {
			t.Fatalf("migration %06d changed on repeat up", version)
		}
	}
	if err := VerifyUpToDate(db); err != nil {
		t.Fatalf("verify after repeat up: %v", err)
	}
}

func TestMigrationChecksumDriftFailsClosed(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUp(db); err != nil {
		t.Fatalf("up: %v", err)
	}

	if err := db.Exec(`UPDATE schema_migrations SET checksum = 'drifted' WHERE version = 9`).Error; err != nil {
		t.Fatalf("drift setup: %v", err)
	}

	mustFailContaining(t, "up with drifted checksum", func() error {
		return MigrateUp(db)
	}, "was modified after being applied")

	mustFailContaining(t, "verify with drifted checksum", func() error {
		return VerifyUpToDate(db)
	}, "modified after being applied")
}

func TestMigrateUpToStagedOrdering(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 3); err != nil {
		t.Fatalf("up to 3: %v", err)
	}
	assertAppliedExactly(t, db, 1, 2, 3)

	if err := MigrateUpTo(db, 6); err != nil {
		t.Fatalf("up to 6: %v", err)
	}
	assertAppliedExactly(t, db, 1, 2, 3, 4, 5, 6)
}

// TestMigrationPartialFailureRollsBack drives 000007 into its fail-closed
// branch: a live Partner user while a Manager role exists must abort the
// migration, leave no tracking record for it, and change nothing.
func TestMigrationPartialFailureRollsBack(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 5); err != nil {
		t.Fatalf("baseline up: %v", err)
	}
	seedMigrationRolesRaw(t, db)

	partnerRole := uuid.New()
	if err := db.Exec(`INSERT INTO roles (id, name) VALUES (?::uuid, 'Partner')`, partnerRole.String()).Error; err != nil {
		t.Fatalf("seed partner role: %v", err)
	}
	partnerUser := migrationInsertUser(t, db, "partner_conflict", partnerRole, nil, "Active")

	mustFailContaining(t, "up with live Partner/Manager conflict", func() error {
		return MigrateUpTo(db, 9)
	}, "000007 fail-closed")

	// Partial-failure semantics: everything before 000007 stays applied
	// (its own committed transaction), 000007 and later are not recorded.
	assertAppliedExactly(t, db, 1, 2, 3, 4, 5, 6)

	var stillPartner int64
	if err := db.Raw(`SELECT count(*) FROM users WHERE id = ?::uuid AND role_id = ?::uuid`, partnerUser.String(), partnerRole.String()).Scan(&stillPartner).Error; err != nil {
		t.Fatalf("partner user lookup: %v", err)
	}
	if stillPartner != 1 {
		t.Fatal("the live Partner user must be untouched by the failed migration")
	}

	var partnerRoles int64
	if err := db.Raw(`SELECT count(*) FROM roles WHERE name = 'Partner'`).Scan(&partnerRoles).Error; err != nil {
		t.Fatalf("partner role lookup: %v", err)
	}
	if partnerRoles != 1 {
		t.Fatal("the Partner role must survive the failed migration")
	}

	// 000006 must have been applied before 000007 failed (own transaction).
	mustHaveColumn(t, db, "users", "organization_id")
}
