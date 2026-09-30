package database

// PHASE 4F — dedicated migration-test harness.
//
// The migration integration tests run against a dedicated throwaway
// PostgreSQL schema (`pwams_migration_test`) inside the configured
// development database. The operator explicitly approved this dedicated
// schema as the migration test target; the canonical `pwams_user` schema
// and the legacy `public` schema are never touched by these tests.
//
// Safety rails:
//   - every test is gated on PWAMS_RUN_MIGRATION_TESTS=1; a plain
//     `go test ./...` never opens a connection from this file;
//   - the gorm pool is pinned to a single connection whose search_path
//     is hard-set to the test schema, and current_schema() is re-asserted
//     before every test, so a pool reconnect can never silently redirect
//     statements into `public` or `pwams_user`;
//   - the `public` + `pwams_user` table inventories are snapshotted and
//     re-checked (TestMigrationsDoNotTouchOtherSchemas).

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
)

const migrationTestSchema = "pwams_migration_test"

var (
	migTestDB       *gorm.DB
	migTestConcurDB *sql.DB
	migTestOnce     sync.Once
	migTestErr      error
)

func migrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("PWAMS_RUN_MIGRATION_TESTS") != "1" {
		t.Skip("set PWAMS_RUN_MIGRATION_TESTS=1 to run the migration suite against the dedicated test schema")
	}

	migTestOnce.Do(func() {
		// Test binaries run from the package directory; the repo-root
		// .env is loaded as a fallback (same convention as the other
		// integration harnesses). Existing env vars are never overridden.
		_ = godotenv.Load("../../.env")

		cfg, err := config.Load()
		if err != nil {
			migTestErr = err
			return
		}

		db, err := Connect(cfg)
		if err != nil {
			migTestErr = fmt.Errorf("connect: %w", err)
			return
		}

		sqlDB, err := db.DB()
		if err != nil {
			migTestErr = err
			return
		}

		// Recreate the dedicated throwaway schema for a clean baseline.
		// Destructive ONLY inside the test schema.
		if _, err := sqlDB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, migrationTestSchema)); err != nil {
			migTestErr = fmt.Errorf("drop test schema: %w", err)
			return
		}
		if _, err := sqlDB.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, migrationTestSchema)); err != nil {
			migTestErr = fmt.Errorf("create test schema: %w", err)
			return
		}

		// Pin a single pooled connection: SET search_path then applies to
		// every statement this gorm handle issues.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		if _, err := sqlDB.Exec(fmt.Sprintf(`SET search_path TO %q`, migrationTestSchema)); err != nil {
			migTestErr = fmt.Errorf("set search_path: %w", err)
			return
		}

		var current string
		if err := db.Raw(`SELECT current_schema()`).Scan(&current).Error; err != nil {
			migTestErr = err
			return
		}
		if current != migrationTestSchema {
			migTestErr = fmt.Errorf("current_schema() = %q, want %q", current, migrationTestSchema)
			return
		}

		// Separate unpinned pool for per-connection setups (the
		// concurrency test needs genuinely parallel sessions).
		concur, err := sql.Open("pgx", buildMigrationTestDSN(cfg))
		if err != nil {
			migTestErr = err
			return
		}
		concur.SetMaxOpenConns(16)

		migTestDB = db
		migTestConcurDB = concur
	})

	if migTestErr != nil {
		t.Skipf("migration integration test DB unavailable: %v", migTestErr)
	}
	if err := assertMigrationTestSchema(migTestDB); err != nil {
		t.Fatalf("test schema assertion failed: %v", err)
	}
	return migTestDB
}

// assertMigrationTestSchema refuses to continue unless the connection
// resolves to the dedicated test schema. Called before every test.
func assertMigrationTestSchema(db *gorm.DB) error {
	var current string
	if err := db.Raw(`SELECT current_schema()`).Scan(&current).Error; err != nil {
		return err
	}
	if current != migrationTestSchema {
		return fmt.Errorf("current_schema() = %q, want %q (refusing to continue)", current, migrationTestSchema)
	}
	return nil
}

// resetMigrationTestSchema recreates the throwaway schema for a clean
// baseline. Destructive ONLY inside the dedicated test schema.
func resetMigrationTestSchema(t *testing.T) {
	t.Helper()
	db := migrationTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("test schema reset: %v", err)
	}
	for _, stmt := range []string{
		fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, migrationTestSchema),
		fmt.Sprintf(`CREATE SCHEMA %q`, migrationTestSchema),
		fmt.Sprintf(`SET search_path TO %q`, migrationTestSchema),
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("test schema reset (%s): %v", stmt, err)
		}
	}
	if err := assertMigrationTestSchema(db); err != nil {
		t.Fatalf("test schema reset verification: %v", err)
	}
}

// tableInventory lists every user table in the legacy public schema and
// the canonical pwams_user schema. Used to prove the migration suite
// performs no cross-schema writes.
func tableInventory(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var rows []string
	if err := db.Raw(`SELECT schemaname || '.' || tablename
		FROM pg_tables
		WHERE schemaname IN ('public', 'pwams_user')
		ORDER BY 1`).Scan(&rows).Error; err != nil {
		t.Fatalf("table inventory: %v", err)
	}
	return rows
}

// newPinnedConn returns a dedicated session whose search_path is the test
// schema. Safe to call from test goroutines (errors are returned, not
// fatal); the caller owns the returned connection.
func newPinnedConn(t *testing.T) (*sql.Conn, error) {
	migrationTestDB(t)
	ctx := context.Background()
	conn, err := migTestConcurDB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`SET search_path TO %q`, migrationTestSchema)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// ─── Seed / assertion helpers (raw SQL — Go models stay untouched until
// ─── the production migration is approved) ───

func migrationRoleID(t *testing.T, db *gorm.DB, name string) uuid.UUID {
	t.Helper()
	// pgx returns the uuid column as text; scan as string and parse so
	// the lookup works on every baseline (the uuid.UUID Scan path is not
	// wired for the raw pgx text rendering in this pinned pool).
	var raw string
	if err := db.Raw(`SELECT id::text FROM roles WHERE name = ?`, name).Scan(&raw).Error; err != nil || raw == "" {
		t.Fatalf("role %q lookup failed: %v", name, err)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("role %q lookup returned unparsable id %q: %v", name, raw, err)
	}
	return id
}

// seedMigrationRolesRaw inserts the standard roles with plain SQL for
// tests that must seed BEFORE 000009 (on the raw 000001 baseline the
// roles table has no description/deleted_at columns yet, so the GORM
// model cannot be used there). UUIDs ride as text with an explicit
// ::uuid cast — pgx cannot infer the type of a [16]byte uuid param.
func seedMigrationRolesRaw(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, name := range []string{
		"Super Admin", "Admin", "Staff", "Volunteer",
		"Donor", "Beneficiary", "Manager", "Student",
	} {
		if err := db.Exec(`INSERT INTO roles (id, name) VALUES (?::uuid, ?) ON CONFLICT (name) DO NOTHING`,
			uuid.New().String(), name).Error; err != nil {
			t.Fatalf("seed role %s: %v", name, err)
		}
	}
}

func migrationCreateOrg(t *testing.T, db *gorm.DB, code string) uuid.UUID {
	t.Helper()
	// Scan RETURNING id as text then parse: database/sql cannot scan a
	// text UUID straight into google's [16]byte uuid type (it attempts a
	// uint8 conversion and fails).
	var raw string
	if err := db.Raw(`INSERT INTO organizations (id, name, code) VALUES (?::uuid, ?, ?) RETURNING id::text`,
		uuid.New().String(), "Org "+code, code).Scan(&raw).Error; err != nil || raw == "" {
		t.Fatalf("create org %s: %v", code, err)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("create org %s returned unparsable id %q: %v", code, raw, err)
	}
	return id
}

// insertUserErr inserts a user row. organization_id is only referenced
// when orgID is non-nil, so the helper also works on the pre-000006
// baseline (where the column does not exist yet).
func insertUserErr(db *gorm.DB, username string, roleID uuid.UUID, orgID *uuid.UUID, status string) (uuid.UUID, error) {
	// UUIDs are passed as text with explicit ::uuid casts (deterministic
	// pgx rendering on every baseline) and RETURNING id is scanned as
	// text, then parsed — database/sql cannot scan text directly into the
	// google [16]byte uuid type.
	var raw string
	var err error
	if orgID == nil {
		err = db.Raw(`INSERT INTO users (id, username, email, password_hash, role_id, status)
			VALUES (?::uuid, ?, ?, 'migration-test-hash', ?::uuid, ?) RETURNING id::text`,
			uuid.New().String(), username, username+"@migration.test", roleID.String(), status).Scan(&raw).Error
	} else {
		err = db.Raw(`INSERT INTO users (id, username, email, password_hash, role_id, status, organization_id)
			VALUES (?::uuid, ?, ?, 'migration-test-hash', ?::uuid, ?, ?::uuid) RETURNING id::text`,
			uuid.New().String(), username, username+"@migration.test", roleID.String(), status, orgID.String()).Scan(&raw).Error
	}
	if err != nil {
		return uuid.Nil, err
	}
	id, perr := uuid.Parse(raw)
	if perr != nil {
		return uuid.Nil, perr
	}
	return id, nil
}

func migrationInsertUser(t *testing.T, db *gorm.DB, username string, roleID uuid.UUID, orgID *uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id, err := insertUserErr(db, username, roleID, orgID, status)
	if err != nil {
		t.Fatalf("insert user %s: %v", username, err)
	}
	return id
}

func setUserStatusErr(db *gorm.DB, id uuid.UUID, status string) error {
	return db.Exec(`UPDATE users SET status = ? WHERE id = ?::uuid`, status, id.String()).Error
}

func setUserDeletedErr(db *gorm.DB, id uuid.UUID) error {
	return db.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = ?::uuid`, id.String()).Error
}

// mustFailContaining asserts fn fails and the error mentions every needle.
func mustFailContaining(t *testing.T, what string, fn func() error, needles ...string) {
	t.Helper()
	err := fn()
	if err == nil {
		t.Fatalf("%s: expected an error, got nil", what)
	}
	for _, needle := range needles {
		if !strings.Contains(err.Error(), needle) {
			t.Fatalf("%s: error %q does not mention %q", what, err.Error(), needle)
		}
	}
}

// buildMigrationTestDSN mirrors the application DSN
// (internal/database/postgres.go) for the parallel-session pool.
func buildMigrationTestDSN(cfg *config.Config) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Colombo",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, cfg.DBSSLMode,
	)
}

func mustHaveColumn(t *testing.T, db *gorm.DB, table, column string) {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		table, column).Scan(&n).Error; err != nil {
		t.Fatalf("column lookup %s.%s: %v", table, column, err)
	}
	if n != 1 {
		t.Fatalf("expected column %s.%s to exist", table, column)
	}
}

func mustNotHaveColumn(t *testing.T, db *gorm.DB, table, column string) {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
		table, column).Scan(&n).Error; err != nil {
		t.Fatalf("column lookup %s.%s: %v", table, column, err)
	}
	if n != 0 {
		t.Fatalf("column %s.%s must not exist", table, column)
	}
}

func hasUniqueIndex(t *testing.T, db *gorm.DB, table, index string) bool {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM pg_indexes
		WHERE schemaname = current_schema() AND tablename = ? AND indexname = ?
		AND indexdef LIKE 'CREATE UNIQUE%'`, table, index).Scan(&n).Error; err != nil {
		t.Fatalf("index lookup %s: %v", index, err)
	}
	return n == 1
}

func hasTrigger(t *testing.T, db *gorm.DB, table, trigger string) bool {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM pg_trigger
		WHERE tgrelid = ?::regclass AND tgname = ? AND NOT tgisinternal`, table, trigger).Scan(&n).Error; err != nil {
		t.Fatalf("trigger lookup %s: %v", trigger, err)
	}
	return n == 1
}

func hasConstraint(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int64
	if err := db.Raw(`SELECT count(*) FROM pg_constraint
		WHERE conname = ? AND connamespace = current_schema()::regnamespace`, name).Scan(&n).Error; err != nil {
		t.Fatalf("constraint lookup %s: %v", name, err)
	}
	return n >= 1
}

func insertPerson(t *testing.T, db *gorm.DB, name string, createdBy uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var raw string
	if err := db.Raw(`INSERT INTO persons (id, full_name, nic_passport, created_by_id)
		VALUES (?::uuid, ?, ?, ?::uuid) RETURNING id::text`,
		id.String(), name, "NIC-"+id.String()[:8], createdBy.String()).Scan(&raw).Error; err != nil || raw == "" {
		t.Fatalf("insert person: %v", err)
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("insert person returned unparsable id %q: %v", raw, err)
	}
	return parsed
}

func insertLoan(t *testing.T, db *gorm.DB, personID, createdBy uuid.UUID) uuid.UUID {
	t.Helper()
	var raw string
	if err := db.Raw(`INSERT INTO loans (id, person_id, loan_amount, duration_months, installment_amount, created_by_id)
		VALUES (?::uuid, ?::uuid, 12000, 12, 1000, ?::uuid) RETURNING id::text`,
		uuid.New().String(), personID.String(), createdBy.String()).Scan(&raw).Error; err != nil || raw == "" {
		t.Fatalf("insert loan: %v", err)
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("insert loan returned unparsable id %q: %v", raw, err)
	}
	return parsed
}

func assertAppliedExactly(t *testing.T, db *gorm.DB, versions ...int64) {
	t.Helper()
	applied, err := AppliedMigrationsReadOnly(db)
	if err != nil {
		t.Fatalf("applied lookup: %v", err)
	}
	if len(applied) != len(versions) {
		t.Fatalf("applied %d migrations, want exactly %v", len(applied), versions)
	}
	for _, v := range versions {
		if _, ok := applied[v]; !ok {
			t.Fatalf("migration %06d not applied", v)
		}
	}
}
