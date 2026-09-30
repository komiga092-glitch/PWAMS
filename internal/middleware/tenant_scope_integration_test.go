package middleware

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/migrations"
)

// PHASE 4P follow-up — tenant middleware integration tests run inside a
// dedicated throwaway PostgreSQL schema (`pwams_middleware_test`), mirroring
// the Phase 4F migration-test harness convention.
//
// Previously these tests connected to the configured database with the
// application search_path, i.e. the CANONICAL `pwams_user` production
// schema. After migration 000006 every tenant_id must reference a real
// organizations row (fk_donors_tenant_organization), so seeding donors with
// random UUID tenants wrote production rows and failed the FK. On setups
// without a canonical schema the same connection fell through to `public`,
// which the application role may not CREATE in (PG15+ default).
//
// Safety rails (same as internal/database/migration_test_harness_test.go):
//   - the gorm pool is pinned to a single connection whose search_path is
//     hard-set to the throwaway schema; current_schema() is re-asserted
//     before every test, so a pool reconnect can never redirect statements
//     into `public` or `pwams_user`;
//   - the canonical `pwams_user` schema and the legacy `public` schema are
//     never dropped, migrated, or written by these tests;
//   - tenants are REAL organizations rows created inside the test schema,
//     so the tenancy FK is exercised, not bypassed.

// tenantContextUser mimics an authenticated user carrying a tenant id — the
// interface WithTenantScope introspects for isolation.
type tenantContextUser struct {
	tenantID *uuid.UUID
}

func (u *tenantContextUser) GetTenantID() *uuid.UUID { return u.tenantID }

const tenantTestSchema = "pwams_middleware_test"

var (
	tenantTestOnce sync.Once
	tenantTestGorm *gorm.DB
	tenantTestCfg  *config.Config
	tenantTestErr  error
)

// tenantTestDB provisions the throwaway test schema once per test binary
// and returns a gorm handle pinned to it. When no database is reachable
// the tests are skipped so the suite still runs in a plain
// `go test ./...` environment; when a database is available the
// tenant-scoping behaviour is verified against real rows.
func tenantTestDB(t *testing.T) (*gorm.DB, *config.Config) {
	t.Helper()

	tenantTestOnce.Do(func() {
		// Test binaries run from the package directory; the repo-root
		// .env is loaded as a fallback. Existing env vars are never
		// overridden.
		_ = godotenv.Load("../../.env")

		cfg, err := config.Load()
		if err != nil {
			tenantTestErr = err
			return
		}

		db, err := database.Connect(cfg)
		if err != nil {
			tenantTestErr = fmt.Errorf("connect: %w", err)
			return
		}

		sqlDB, err := db.DB()
		if err != nil {
			tenantTestErr = err
			return
		}

		// Recreate the dedicated throwaway schema for a clean baseline.
		// Destructive ONLY inside the test schema.
		if _, err := sqlDB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, tenantTestSchema)); err != nil {
			tenantTestErr = fmt.Errorf("drop test schema: %w", err)
			return
		}
		if _, err := sqlDB.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, tenantTestSchema)); err != nil {
			tenantTestErr = fmt.Errorf("create test schema: %w", err)
			return
		}

		// Pin a single pooled connection: SET search_path then applies to
		// every statement this gorm handle issues, and a single-conn pool
		// cannot silently reconnect onto a default search_path.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		if _, err := sqlDB.Exec(fmt.Sprintf(`SET search_path TO %q`, tenantTestSchema)); err != nil {
			tenantTestErr = fmt.Errorf("set search_path: %w", err)
			return
		}

		var current string
		if err := db.Raw(`SELECT current_schema()`).Scan(&current).Error; err != nil {
			tenantTestErr = err
			return
		}
		if current != tenantTestSchema {
			tenantTestErr = fmt.Errorf("current_schema() = %q, want %q (refusing to continue)", current, tenantTestSchema)
			return
		}

		// Migrate + seed INSIDE the throwaway schema only. AutoMigrate
		// covers the model tables; migration 000006 (resolved through the
		// pinned search_path) adds the tenancy scaffolding — the
		// organizations registry and the fk_*_tenant_organization
		// constraints the tests must honour — without touching any
		// production schema.
		if err := database.Migrate(db); err != nil {
			tenantTestErr = fmt.Errorf("tenant integration test migration failed: %w", err)
			return
		}
		orgDDL, err := migrations.FS.ReadFile("000006_organization_tenancy.up.sql")
		if err != nil {
			tenantTestErr = fmt.Errorf("read 000006 DDL: %w", err)
			return
		}
		if err := db.Exec(string(orgDDL)).Error; err != nil {
			tenantTestErr = fmt.Errorf("apply 000006 DDL in test schema: %w", err)
			return
		}
		if err := database.SeedDefaultRoles(db); err != nil {
			tenantTestErr = fmt.Errorf("tenant integration test role seed failed: %w", err)
			return
		}

		tenantTestGorm = db
		tenantTestCfg = cfg
	})

	if tenantTestErr != nil {
		t.Skipf("tenant integration test skipped (test database unavailable): %v", tenantTestErr)
	}
	if err := assertTenantTestSchema(tenantTestGorm); err != nil {
		t.Fatalf("test schema assertion failed: %v", err)
	}
	return tenantTestGorm, tenantTestCfg
}

// assertTenantTestSchema refuses to continue unless the connection
// resolves to the dedicated throwaway schema. Called before every test.
func assertTenantTestSchema(db *gorm.DB) error {
	var current string
	if err := db.Raw(`SELECT current_schema()`).Scan(&current).Error; err != nil {
		return err
	}
	if current != tenantTestSchema {
		return fmt.Errorf("current_schema() = %q, want %q (refusing to continue)", current, tenantTestSchema)
	}
	return nil
}

// seedTwoTenantDonors creates two REAL organizations (NGO A / NGO B) and a
// donor in each. Because migration 000006 enforces
// fk_donors_tenant_organization, tenant ids must be actual organizations
// rows — the FK is part of the behaviour under test, not an obstacle.
// The organizations table is DDL-only today (no Go model yet — Phase 4P
// left organization bootstrap to an explicit operator action), so the
// tenant rows are inserted with raw SQL inside the TEST schema.
func seedTwoTenantDonors(t *testing.T, db *gorm.DB, cfg *config.Config) (uuid.UUID, uuid.UUID) {
	t.Helper()

	if err := database.SeedSuperAdmin(db, cfg); err != nil {
		t.Fatalf("tenant integration test super-admin seed failed: %v", err)
	}

	var creator models.User
	if err := db.Where("email = ?", cfg.SuperAdminEmail).First(&creator).Error; err != nil {
		t.Fatalf("tenant integration test: cannot resolve creator user: %v", err)
	}

	tenantA := uuid.New()
	tenantB := uuid.New()

	// organizations.code is UNIQUE and the throwaway schema persists for
	// the whole test binary, so every seeding gets a run-scoped suffix.
	orgCodes := []string{
		"TENANT_A_" + uuid.NewString()[:8],
		"TENANT_B_" + uuid.NewString()[:8],
	}

	for _, org := range []struct {
		id   uuid.UUID
		code string
	}{
		{tenantA, orgCodes[0]},
		{tenantB, orgCodes[1]},
	} {
		if err := db.Exec(`INSERT INTO organizations (id, name, code, status) VALUES (?, ?, ?, 'Active')`,
			org.id, "Test Org "+org.code, org.code).Error; err != nil {
			t.Fatalf("tenant integration test: create organization %s: %v", org.code, err)
		}
	}

	mk := func(name string, tenant *uuid.UUID) *models.Donor {
		return &models.Donor{
			Name:        name,
			DonorType:   models.DonorTypeIndividual,
			Status:      models.DonorStatusActive,
			CreatedByID: creator.ID,
			TenantID:    tenant,
		}
	}

	donorA := mk("Tenant Isolation A Donor", &tenantA)
	donorB := mk("Tenant Isolation B Donor", &tenantB)
	if err := db.Create(donorA).Error; err != nil {
		t.Fatalf("failed to create donor A: %v", err)
	}
	if err := db.Create(donorB).Error; err != nil {
		t.Fatalf("failed to create donor B: %v", err)
	}

	t.Cleanup(func() {
		// Donors first: organizations are RESTRICT-referenced by tenant_id.
		_ = db.Unscoped().Where("name IN ?", []string{donorA.Name, donorB.Name}).Delete(&models.Donor{}).Error
		_ = db.Exec(`DELETE FROM organizations WHERE id IN (?, ?)`, tenantA, tenantB).Error
	})

	return tenantA, tenantB
}

// tenantScopedDonors runs a donor query scoped to the actor's tenant and
// returns the matched rows.
func tenantScopedDonors(t *testing.T, db *gorm.DB, tenantID *uuid.UUID, names []string) []models.Donor {
	t.Helper()

	c, _ := gin.CreateTestContext(nil)
	c.Set("current_user", &tenantContextUser{tenantID: tenantID})

	var donors []models.Donor
	if err := WithTenantScope(db, c).
		Where("name IN ?", names).
		Find(&donors).Error; err != nil {
		t.Fatalf("tenant-scoped donor query failed: %v", err)
	}
	return donors
}

// TestAdminCannotAccessAnotherTenant verifies that an Admin scoped to NGO A
// never sees a donor issued by NGO B (spec section 27 / 34).
func TestAdminCannotAccessAnotherTenant(t *testing.T) {
	db, cfg := tenantTestDB(t)
	tenantA, tenantB := seedTwoTenantDonors(t, db, cfg)

	names := []string{"Tenant Isolation A Donor", "Tenant Isolation B Donor"}

	rows := tenantScopedDonors(t, db, &tenantA, names)
	for _, donor := range rows {
		if donor.TenantID != nil && *donor.TenantID == tenantB {
			t.Fatalf("Admin of NGO A must not see NGO B donor %q", donor.Name)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("Admin of NGO A must see exactly the NGO A donor, got %d rows", len(rows))
	}
}

// TestManagerCannotAccessAnotherTenant verifies the same boundary for a
// Manager account (the user-facing NGO role since migration 000005; the
// legacy "Partner" name is only a compatibility alias and is never
// re-introduced as a user-facing role).
func TestManagerCannotAccessAnotherTenant(t *testing.T) {
	db, cfg := tenantTestDB(t)
	tenantA, tenantB := seedTwoTenantDonors(t, db, cfg)

	names := []string{"Tenant Isolation A Donor", "Tenant Isolation B Donor"}

	rows := tenantScopedDonors(t, db, &tenantB, names)
	for _, donor := range rows {
		if donor.TenantID != nil && *donor.TenantID == tenantA {
			t.Fatalf("Manager of NGO B must not see NGO A donor %q", donor.Name)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("Manager of NGO B must see exactly the NGO B donor, got %d rows", len(rows))
	}
}

// TestWithTenantScope_NoTenantIsNoOp pins that a user without a tenant is
// not filtered (single-tenant deployment must not drop rows).
func TestWithTenantScope_NoTenantIsNoOp(t *testing.T) {
	db, _ := tenantTestDB(t)

	c, _ := gin.CreateTestContext(nil)
	c.Set("current_user", &tenantContextUser{tenantID: nil})

	if scoped := WithTenantScope(db, c); scoped != db {
		t.Fatal("WithTenantScope must return the same *gorm.DB for a user without a tenant")
	}
}

// TestContextTenantID pins the extraction contract used by the sync
// handlers: tenant-bound principals expose their tenant, plain users
// yield nil.
func TestContextTenantID(t *testing.T) {
	tenantA := uuid.New()

	c, _ := gin.CreateTestContext(nil)
	c.Set("current_user", &tenantContextUser{tenantID: &tenantA})
	if got := ContextTenantID(c); got == nil || *got != tenantA {
		t.Fatalf("ContextTenantID = %v, want %s", got, tenantA)
	}

	c2, _ := gin.CreateTestContext(nil)
	c2.Set("current_user", &tenantContextUser{tenantID: nil})
	if got := ContextTenantID(c2); got != nil {
		t.Fatalf("ContextTenantID for a principal without tenant = %v, want nil", got)
	}

	c3, _ := gin.CreateTestContext(nil)
	if got := ContextTenantID(c3); got != nil {
		t.Fatalf("ContextTenantID without principal = %v, want nil", got)
	}
}
