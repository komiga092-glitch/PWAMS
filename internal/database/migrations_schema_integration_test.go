package database

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PHASE 4F — schema, data-safety and tenant-ownership behaviour of the
// migration set, verified inside the dedicated migration test schema.

// TestExistingDataPreservedThroughTenancyMigrations seeds representative
// data on the pre-tenancy baseline (000001-000005) and proves 000006-000009
// preserve every row, assign nobody to any organization and never fabricate
// one. Mirrors the canonical deployment (581 users / 237 persons, all
// tenant_id NULL).
func TestExistingDataPreservedThroughTenancyMigrations(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 5); err != nil {
		t.Fatalf("baseline up: %v", err)
	}
	seedMigrationRolesRaw(t, db)

	adminRole := migrationRoleID(t, db, "Admin")
	staffRole := migrationRoleID(t, db, "Staff")
	adminID := migrationInsertUser(t, db, "preserve_admin", adminRole, nil, "Active")
	staffID := migrationInsertUser(t, db, "preserve_staff", staffRole, nil, "Active")

	person1 := insertPerson(t, db, "Preserved One", adminID)
	person2 := insertPerson(t, db, "Preserved Two", adminID)
	_ = staffID

	// 000006 and 000007 do not fabricate organizations (migration files end
	// with a single audit_logs marker INSERT each — see
	// migrations/000006_organization_tenancy.up.sql and
	// migrations/000007_reconcile_partner_manager.up.sql). Record only the
	// data-table baselines here; the marker rows are asserted separately
	// below so an audit_trigger never enters the picture.
	countsBefore := map[string]int64{}
	for _, table := range []string{"users", "persons", "roles", "audit_logs"} {
		var n int64
		if err := db.Raw(`SELECT count(*) FROM ` + table).Scan(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		countsBefore[table] = n
	}
	if err := MigrateUpTo(db, 9); err != nil {
		t.Fatalf("tenancy up: %v", err)
	}

	// Data tables keep every row; audit_logs legitimately gains the two
	// migration marker rows (000006 tenancy scaffolding + 000007 no-op
	// reconciliation) documented in the migration files.
	for table, want := range countsBefore {
		var n int64
		if err := db.Raw(`SELECT count(*) FROM ` + table).Scan(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		want := want
		if table == "audit_logs" {
			want += 2
		}
		if n != want {
			t.Fatalf("%s row count changed: before=%d after=%d", table, countsBefore[table], n)
		}
	}

	for _, id := range []uuid.UUID{person1, person2} {
		var n int64
		if err := db.Raw(`SELECT count(*) FROM persons WHERE id = ?::uuid AND tenant_id IS NULL`, id.String()).Scan(&n).Error; err != nil {
			t.Fatalf("person lookup: %v", err)
		}
		if n != 1 {
			t.Fatalf("person %s lost or tenant-assigned", id)
		}
	}
	for _, id := range []uuid.UUID{adminID, staffID} {
		var n int64
		if err := db.Raw(`SELECT count(*) FROM users WHERE id = ?::uuid AND organization_id IS NULL`, id.String()).Scan(&n).Error; err != nil {
			t.Fatalf("user lookup: %v", err)
		}
		if n != 1 {
			t.Fatalf("user %s lost or organisation-assigned", id)
		}
	}

	// No fabricated organizations and no fabricated assignments.
	var orgs int64
	if err := db.Raw(`SELECT count(*) FROM organizations`).Scan(&orgs).Error; err != nil {
		t.Fatalf("organizations count: %v", err)
	}
	if orgs != 0 {
		t.Fatalf("organizations table must stay empty until an explicit bootstrap, got %d", orgs)
	}

	// Tenancy constraints exist.
	for _, name := range []string{
		"fk_users_organization",
		"fk_persons_tenant_organization",
		"fk_donations_tenant_organization",
		"fk_audit_logs_tenant_organization",
		"fk_loans_tenant_organization",
	} {
		if !hasConstraint(t, db, name) {
			t.Fatalf("constraint %s missing", name)
		}
	}

	// Auditability markers from 000006 and 000007 (the merge path writes
	// exactly one 000007 marker; the rename path also writes one).
	for _, prefix := range []string{"000006:", "000007:"} {
		var markers int64
		if err := db.Raw(`SELECT count(*) FROM audit_logs WHERE action = 'migration' AND details LIKE ?`, prefix+"%").Scan(&markers).Error; err != nil {
			t.Fatalf("audit marker lookup %s: %v", prefix, err)
		}
		if markers != 1 {
			t.Fatalf("expected exactly one %s audit marker, got %d", prefix, markers)
		}
	}
}

// TestTenantForeignKeyEnforcement proves the tenant ownership constraints
// introduced by 000006 reject dangling tenant references and accept real
// organization ids.
func TestTenantForeignKeyEnforcement(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 9); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := SeedDefaultRoles(db); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	adminRole := migrationRoleID(t, db, "Admin")
	orgA := migrationCreateOrg(t, db, "fk-a")

	// Dangling tenant_id is rejected on audit_logs.
	mustFailContaining(t, "audit tenant FK", func() error {
		return db.Exec(`INSERT INTO audit_logs (id, action, entity, tenant_id) VALUES (?::uuid, 'test', 'test', ?::uuid)`,
			uuid.New().String(), uuid.New().String()).Error
	}, "violates foreign key constraint")

	// A real organization id is accepted.
	if err := db.Exec(`INSERT INTO audit_logs (id, action, entity, tenant_id) VALUES (?::uuid, 'test', 'test', ?::uuid)`,
		uuid.New().String(), orgA.String()).Error; err != nil {
		t.Fatalf("valid tenant insert: %v", err)
	}

	// Dangling users.organization_id is rejected; a real one is accepted.
	mustFailContaining(t, "users organization FK", func() error {
		return db.Exec(`INSERT INTO users (id, username, email, password_hash, role_id, status, organization_id)
			VALUES (?::uuid, 'org_dangle', 'org_dangle@migration.test', 'x', ?::uuid, 'Active', ?::uuid)`,
			uuid.New().String(), adminRole.String(), uuid.New().String()).Error
	}, "violates foreign key constraint")

	if err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role_id, status, organization_id)
		VALUES (?::uuid, 'org_ok', 'org_ok@migration.test', 'x', ?::uuid, 'Active', ?::uuid)`,
		uuid.New().String(), adminRole.String(), orgA.String()).Error; err != nil {
		t.Fatalf("valid organization assignment: %v", err)
	}
}

// TestUniqueConstraintsAfterMigrations verifies the uniqueness guarantees a
// migration-only deployment must have: organization code, username, and the
// composite loan repayment installment index.
func TestUniqueConstraintsAfterMigrations(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 9); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := SeedDefaultRoles(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	adminRole := migrationRoleID(t, db, "Admin")
	adminID := migrationInsertUser(t, db, "uniq_admin", adminRole, nil, "Active")

	// organizations.code
	if err := db.Exec(`INSERT INTO organizations (id, name, code) VALUES (?::uuid, 'A', 'shared-code')`, uuid.New().String()).Error; err != nil {
		t.Fatalf("first org: %v", err)
	}
	mustFailContaining(t, "duplicate organization code", func() error {
		return db.Exec(`INSERT INTO organizations (id, name, code) VALUES (?::uuid, 'B', 'shared-code')`, uuid.New().String()).Error
	}, "organizations_code_key")

	// users.username
	mustFailContaining(t, "duplicate username", func() error {
		return db.Exec(`INSERT INTO users (id, username, email, password_hash, role_id, status)
			VALUES (?::uuid, 'uniq_admin', 'other@migration.test', 'x', ?::uuid, 'Active')`, uuid.New().String(), adminRole.String()).Error
	}, "users_username_key")

	// loan_repayments (loan_id, installment_number)
	personID := insertPerson(t, db, "Loan Person", adminID)
	loanID := insertLoan(t, db, personID, adminID)
	// amount/payment_date/created_by_id are legacy NOT NULL columns of the
	// original one-shot repayment design (000001); they are NOT NULL on a
	// migration-only baseline, so the test must supply them.
	if err := db.Exec(`INSERT INTO loan_repayments (id, loan_id, installment_number, due_date, payment_date, amount, created_by_id)
		VALUES (?::uuid, ?::uuid, 1, NOW() + interval '30 days', NOW(), 1000, ?::uuid)`, uuid.New().String(), loanID.String(), adminID.String()).Error; err != nil {
		t.Fatalf("first repayment: %v", err)
	}
	mustFailContaining(t, "duplicate installment", func() error {
		return db.Exec(`INSERT INTO loan_repayments (id, loan_id, installment_number, due_date, payment_date, amount, created_by_id)
			VALUES (?::uuid, ?::uuid, 1, NOW() + interval '30 days', NOW(), 1000, ?::uuid)`, uuid.New().String(), loanID.String(), adminID.String()).Error
	}, "uk_loan_repayment_installment")
}

// TestOneActiveManagerPerOrganization verifies the 000008 trigger:
// (organization_id, Manager role, Active status) has at most one row per
// organization, including via role changes, disable/replace/re-enable
// flows and soft-deletes.
func TestOneActiveManagerPerOrganization(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 9); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := SeedDefaultRoles(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	managerRole := migrationRoleID(t, db, "Manager")
	staffRole := migrationRoleID(t, db, "Staff")
	orgA := migrationCreateOrg(t, db, "mgr-a")
	orgB := migrationCreateOrg(t, db, "mgr-b")

	// 1. First active Manager succeeds.
	managerA := migrationInsertUser(t, db, "mgr_a_1", managerRole, &orgA, "Active")

	// 2. A second active Manager for the same organization fails.
	mustFailContaining(t, "second manager", func() error {
		_, err := insertUserErr(db, "mgr_a_2", managerRole, &orgA, "Active")
		return err
	}, "one active Manager per organization")

	// 3. A disabled Manager does not conflict.
	if _, err := insertUserErr(db, "mgr_a_disabled", managerRole, &orgA, "Disabled"); err != nil {
		t.Fatalf("disabled manager must not conflict: %v", err)
	}

	// 4. Another organization still gets its own Manager.
	if _, err := insertUserErr(db, "mgr_b_1", managerRole, &orgB, "Active"); err != nil {
		t.Fatalf("organization B manager: %v", err)
	}

	// 5. Disable the only active Manager, then a replacement is allowed.
	if err := setUserStatusErr(db, managerA, "Disabled"); err != nil {
		t.Fatalf("disable manager A: %v", err)
	}
	replacement := migrationInsertUser(t, db, "mgr_a_replacement", managerRole, &orgA, "Active")

	// 6. Reactivating the previous Manager is rejected while the
	// replacement is active.
	mustFailContaining(t, "reactivation conflict", func() error {
		return setUserStatusErr(db, managerA, "Active")
	}, "one active Manager per organization")

	// 7. Soft-deleting the replacement frees the slot again.
	if err := setUserDeletedErr(db, replacement); err != nil {
		t.Fatalf("soft-delete replacement: %v", err)
	}
	if _, err := insertUserErr(db, "mgr_a_3", managerRole, &orgA, "Active"); err != nil {
		t.Fatalf("replacement after soft delete: %v", err)
	}

	// 8. Promoting an existing active user to Manager is also gated.
	staffID := migrationInsertUser(t, db, "mgr_wannabe", staffRole, &orgA, "Active")
	mustFailContaining(t, "role change to manager", func() error {
		return db.Exec(`UPDATE users SET role_id = ?::uuid WHERE id = ?::uuid`, managerRole.String(), staffID.String()).Error
	}, "one active Manager per organization")

	// 9. Platform-scope rows (organization_id NULL) are intentionally not
	// constrained by the trigger — the Phase 2 service-level global rule
	// governs them (backwards compatibility with the current deployment).
	if _, err := insertUserErr(db, "platform_mgr_1", managerRole, nil, "Active"); err != nil {
		t.Fatalf("platform manager 1: %v", err)
	}
	if _, err := insertUserErr(db, "platform_mgr_2", managerRole, nil, "Active"); err != nil {
		t.Fatalf("platform manager 2: %v", err)
	}
}

// TestPartnerMergePathReconciliation exercises the 000007 merge branch that
// production migrations actually take when a canonical Manager already
// exists: the orphan Partner role is merged, soft-deleted Partner accounts
// are re-pointed to Manager, no live user changes role, no account is
// deleted and the reconciliation is audit-logged. Regression test for the
// format()-specifier failure found during the PHASE 4P staged rollout.
func TestPartnerMergePathReconciliation(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 5); err != nil {
		t.Fatalf("baseline up: %v", err)
	}
	seedMigrationRolesRaw(t, db) // standard roles incl. Manager, no Partner

	// Orphan Partner role + a soft-deleted account holding it.
	partnerRole := uuid.New()
	if err := db.Exec(`INSERT INTO roles (id, name) VALUES (?::uuid, 'Partner')`, partnerRole.String()).Error; err != nil {
		t.Fatalf("seed partner role: %v", err)
	}
	managerRole := migrationRoleID(t, db, "Manager")
	deletedPartner := uuid.New()
	if err := db.Exec(`INSERT INTO users (id, username, email, password_hash, role_id, status, deleted_at)
		VALUES (?::uuid, 'legacy_partner', 'legacy_partner@migration.test', 'x', ?::uuid, 'Active', NOW())`,
		deletedPartner.String(), partnerRole.String()).Error; err != nil {
		t.Fatalf("seed partner user: %v", err)
	}
	liveManager := migrationInsertUser(t, db, "merge_path_manager", managerRole, nil, "Active")

	if err := MigrateUpTo(db, 7); err != nil {
		t.Fatalf("reconcile up: %v", err)
	}

	var partnerRoles int64
	if err := db.Raw(`SELECT count(*) FROM roles WHERE name = 'Partner'`).Scan(&partnerRoles).Error; err != nil {
		t.Fatalf("partner lookup: %v", err)
	}
	if partnerRoles != 0 {
		t.Fatalf("orphan Partner role must be merged away, found %d", partnerRoles)
	}

	// The soft-deleted account survives and now holds the canonical Manager.
	var stillExists int64
	if err := db.Raw(`SELECT count(*) FROM users WHERE id = ?::uuid AND deleted_at IS NOT NULL`,
		deletedPartner.String()).Scan(&stillExists).Error; err != nil {
		t.Fatalf("deleted user lookup: %v", err)
	}
	if stillExists != 1 {
		t.Fatal("soft-deleted Partner account must never be deleted by the reconciliation")
	}
	var newRole string
	if err := db.Raw(`SELECT r.name FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = ?::uuid`,
		deletedPartner.String()).Scan(&newRole).Error; err != nil {
		t.Fatalf("role lookup: %v", err)
	}
	if newRole != "Manager" {
		t.Fatalf("soft-deleted account must be re-pointed to Manager, got %q", newRole)
	}

	// The live Manager is untouched; reconciliation is audit-logged.
	var liveRole string
	if err := db.Raw(`SELECT r.name FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = ?::uuid`,
		liveManager.String()).Scan(&liveRole).Error; err != nil {
		t.Fatalf("live manager lookup: %v", err)
	}
	if liveRole != "Manager" {
		t.Fatalf("live Manager must be untouched, got %q", liveRole)
	}
	var markers int64
	if err := db.Raw(`SELECT count(*) FROM audit_logs WHERE action = 'migration' AND details LIKE '000007: merged%'`).Scan(&markers).Error; err != nil {
		t.Fatalf("marker lookup: %v", err)
	}
	if markers != 1 {
		t.Fatalf("reconciliation must be audit-logged exactly once, found %d", markers)
	}
}

// TestManagerUniquenessConcurrentAssignments races parallel sessions to
// create the Manager of one organization and proves the advisory-lock
// trigger lets exactly one win.
func TestManagerUniquenessConcurrentAssignments(t *testing.T) {
	db := migrationTestDB(t)
	resetMigrationTestSchema(t)

	if err := MigrateUpTo(db, 9); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := SeedDefaultRoles(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	managerRole := migrationRoleID(t, db, "Manager")
	orgC := migrationCreateOrg(t, db, "mgr-concurrent")

	const workers = 8
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := newPinnedConn(t)
			if err != nil {
				errs <- err
				return
			}
			defer func() { _ = conn.Close() }()
			// database/sql + pgx cannot encode the google/uuid [16]byte array
			// as a uuid argument, so ids ride as text with explicit casts.
			_, err = conn.ExecContext(context.Background(),
				`INSERT INTO users (id, username, email, password_hash, role_id, status, organization_id)
				 VALUES ($1::uuid, $2, $3, 'x', $4::uuid, 'Active', $5::uuid)`,
				uuid.New().String(), fmt.Sprintf("mgr_race_%d", i),
				fmt.Sprintf("mgr_race_%d@migration.test", i), managerRole.String(), orgC.String())
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	successes := 0
	failures := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if strings.Contains(err.Error(), "one active Manager per organization") {
			failures++
		} else {
			t.Fatalf("unexpected worker error: %v", err)
		}
	}
	if successes != 1 || failures != workers-1 {
		t.Fatalf("expected exactly 1 winner and %d rejections, got %d successes / %d rejections",
			workers-1, successes, failures)
	}

	var active int64
	if err := db.Raw(`SELECT count(*)
		FROM users u JOIN roles r ON r.id = u.role_id
		WHERE r.name = 'Manager' AND u.status = 'Active'
		  AND u.deleted_at IS NULL AND u.organization_id = ?::uuid`, orgC.String()).Scan(&active).Error; err != nil {
		t.Fatalf("active manager count: %v", err)
	}
	if active != 1 {
		t.Fatalf("expected exactly 1 active Manager after the race, got %d", active)
	}
}

// TestMigrationsDoNotTouchOtherSchemas proves the migration suite performs
// no writes outside the dedicated test schema: the legacy `public` and the
// canonical `pwams_user` inventories are identical before/after a full
// migrate cycle, no migration SQL references a hardcoded schema, and the
// tracking table never appears outside the test schema.
func TestMigrationsDoNotTouchOtherSchemas(t *testing.T) {
	db := migrationTestDB(t)

	before := tableInventory(t, db)
	// The tracking table must never be CREATED by this test run outside the
	// test schema. Since PHASE 4P the production database legitimately owns
	// pwams_user.schema_migrations (created by the operator migration run),
	// so the assertion is delta-based: the count of foreign tracking tables
	// must be identical before and after the migrate cycle. (Creation inside
	// the cycle would also be caught by the before/after inventory above.)
	foreignTrackingBefore := foreignTrackingTableCount(t, db)

	resetMigrationTestSchema(t)
	if err := MigrateUp(db); err != nil {
		t.Fatalf("up: %v", err)
	}
	after := tableInventory(t, db)

	if len(before) != len(after) {
		t.Fatalf("table inventory of public/pwams_user changed: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("schema inventory changed: %q -> %q", before[i], after[i])
		}
	}

	files, err := LoadMigrationFiles()
	if err != nil {
		t.Fatalf("files: %v", err)
	}
	for _, file := range files {
		if strings.Contains(file.UpSQL, "public.") || strings.Contains(file.UpSQL, "pwams_user.") {
			t.Fatalf("migration %06d references a hardcoded schema (must resolve through search_path)", file.Version)
		}
	}

	if got := foreignTrackingTableCount(t, db); got != foreignTrackingBefore {
		t.Fatalf("schema_migrations created outside the test schema during migration: %d -> %d", foreignTrackingBefore, got)
	}
}

func foreignTrackingTableCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM pg_tables
		WHERE schemaname IN ('public', 'pwams_user') AND tablename = 'schema_migrations'`).Scan(&n).Error; err != nil {
		t.Fatalf("tracking lookup: %v", err)
	}
	return n
}
