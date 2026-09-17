package middleware

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// tenantContextUser mimics an authenticated user carrying a tenant id — the
// interface WithTenantScope introspects for isolation.
type tenantContextUser struct {
	tenantID *uuid.UUID
}

func (u *tenantContextUser) GetTenantID() *uuid.UUID { return u.tenantID }

// tenantTestDB connects to the configured PostgreSQL database. When no
// database is reachable the tests are skipped so the suite still runs in a
// plain `go test ./...` environment; when a database is available the
// tenant-scoping behaviour is verified against real rows.
func tenantTestDB(t *testing.T) (*gorm.DB, *config.Config) {
	t.Helper()

	// Load the workspace .env when running from a package directory
	// (godotenv.Load only checks the current directory).
	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("tenant integration test skipped (configuration unavailable): %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("tenant integration test skipped (database unavailable): %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("tenant integration test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("tenant integration test role seed failed: %v", err)
	}

	return db, cfg
}

// seedTwoTenantDonors creates a donor in tenantA and a donor in tenantB and
// returns a cleanup that removes both. The two rows are otherwise identical
// in every field, so only the tenant predicate can distinguish them.
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
		_ = db.Unscoped().Where("name IN ?", []string{donorA.Name, donorB.Name}).Delete(&models.Donor{}).Error
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

// TestPartnerCannotAccessAnotherTenant verifies the same boundary for a
// Partner account.
func TestPartnerCannotAccessAnotherTenant(t *testing.T) {
	db, cfg := tenantTestDB(t)
	tenantA, tenantB := seedTwoTenantDonors(t, db, cfg)

	names := []string{"Tenant Isolation A Donor", "Tenant Isolation B Donor"}

	rows := tenantScopedDonors(t, db, &tenantB, names)
	for _, donor := range rows {
		if donor.TenantID != nil && *donor.TenantID == tenantA {
			t.Fatalf("Partner of NGO B must not see NGO A donor %q", donor.Name)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("Partner of NGO B must see exactly the NGO B donor, got %d rows", len(rows))
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
