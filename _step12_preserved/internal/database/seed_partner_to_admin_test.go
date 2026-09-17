package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// partnerToAdminTestDB connects to the configured PostgreSQL database and
// brings it to the canonical state (migrations + role seed + permission seed +
// super admin seed). When no database is reachable the tests are skipped so
// the suite still runs in a plain `go test ./...` environment.
func partnerToAdminTestDB(t *testing.T) (*gorm.DB, *config.Config) {
	t.Helper()

	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("partner-to-admin integration test skipped (configuration unavailable): %v", err)
	}

	db, err := Connect(cfg)
	if err != nil {
		t.Skipf("partner-to-admin integration test skipped (database unavailable): %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("partner-to-admin integration test migration failed: %v", err)
	}
	if err := SeedDefaultRoles(db); err != nil {
		t.Fatalf("partner-to-admin integration test role seed failed: %v", err)
	}
	permSvc := services.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewRoleRepository(db),
		repository.NewAuditLogRepository(db),
	)
	if err := permSvc.SeedDefaults(); err != nil {
		t.Fatalf("partner-to-admin integration test permission seed failed: %v", err)
	}
	if err := SeedSuperAdmin(db, cfg); err != nil {
		t.Fatalf("partner-to-admin integration test super-admin seed failed: %v", err)
	}

	// Clean up any existing Partner (Manager) accounts from previous test runs
	// so each test starts from a known state.
	var partnerRole models.Role
	if err := db.Where("name = ?", models.RolePartner).First(&partnerRole).Error; err == nil {
		var users []models.User
		db.Unscoped().Where("role_id = ?", partnerRole.ID).Find(&users)
		for _, u := range users {
			_ = db.Where("user_id = ?", u.ID).Delete(&models.AuditLog{}).Error
			_ = db.Where("entity_id = ?", u.ID).Delete(&models.AuditLog{}).Error
			_ = db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.PasswordResetToken{}).Error
			_ = db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.Session{}).Error
			_ = db.Exec("DELETE FROM account_activation_tokens WHERE user_id = ?", u.ID).Error
		}
		_ = db.Unscoped().Where("role_id = ?", partnerRole.ID).Delete(&models.User{}).Error
	}

	return db, cfg
}

// seedTestRoleByName resolves a role record by its exact seeded name.
func seedTestRoleByName(t *testing.T, db *gorm.DB, name string) models.Role {
	t.Helper()
	var role models.Role
	if err := db.Where("name = ?", name).First(&role).Error; err != nil {
		t.Fatalf("cannot resolve role %q: %v", name, err)
	}
	return role
}

// seedTestUser inserts a user directly (no activation flow) and registers
// cleanup that removes the account together with the audit rows referencing
// it, so repeated runs never collide on the unique email/username indexes.
func seedTestUser(t *testing.T, db *gorm.DB, roleID uuid.UUID, email string) models.User {
	t.Helper()

	user := models.User{
		Username:     "seedtest_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Email:        email,
		FullName:     "Seed Test Account",
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		RoleID:       roleID,
		Status:       models.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create seed test user: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Where("entity_id = ?", user.ID).Delete(&models.AuditLog{}).Error
		_ = db.Where("user_id = ?", user.ID).Delete(&models.AuditLog{}).Error
		_ = db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{}).Error
	})

	return user
}

// reloadSeedTestUser re-reads the account from the database.
func reloadSeedTestUser(t *testing.T, db *gorm.DB, id uuid.UUID) models.User {
	t.Helper()
	var user models.User
	if err := db.Preload("Role").Where("id = ?", id).First(&user).Error; err != nil {
		t.Fatalf("failed to reload seed test user: %v", err)
	}
	return user
}

// conversionAuditCount counts PARTNER_TO_ADMIN_ROLE_CHANGE audit rows for the
// given target account.
func conversionAuditCount(t *testing.T, db *gorm.DB, targetID uuid.UUID) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&models.AuditLog{}).
		Where("action = ?", "PARTNER_TO_ADMIN_ROLE_CHANGE").
		Where("entity_id = ?", targetID).
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count conversion audit rows: %v", err)
	}
	return count
}

// TestIntegration_PartnerToAdminConversion converts a Partner account to
// Admin: only RoleID changes, every other attribute (ID, username, email,
// password hash, status, created_at) is preserved, the audit record captures
// actor / old role / new role, and every other Partner account is untouched.
func TestIntegration_PartnerToAdminConversion(t *testing.T) {
	db, cfg := partnerToAdminTestDB(t)

	adminRole := seedTestRoleByName(t, db, models.RoleAdmin)
	partnerRole := seedTestRoleByName(t, db, models.RolePartner)

	// The intended account (unique email, exactly one match possible).
	// The second Manager (Partner role) account is created AFTER the first
	// is converted to Admin, because the one-Manager-per-organisation rule
	// (enforced by a partial unique index) allows only one active Partner at
	// a time.
	intendedEmail := "ptoa-intended-" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@pwams.local"

	created := seedTestUser(t, db, partnerRole.ID, intendedEmail)

	// Capture the DB-truncated baseline (a plain reload normalises time
	// precision before the comparison).
	baseline := reloadSeedTestUser(t, db, created.ID)

	// Actor: the seeded Super Admin (operator / system owner).
	var superAdmin models.User
	if err := db.Where("LOWER(email) = ?", strings.ToLower(cfg.SuperAdminEmail)).First(&superAdmin).Error; err != nil {
		t.Fatalf("cannot resolve seeded super admin: %v", err)
	}

	seedCfg := *cfg
	seedCfg.PartnerToAdminEmail = intendedEmail

	converted, err := PromoteConfiguredPartnerToAdmin(
		db,
		&seedCfg,
		services.NewAuditLogService(repository.NewAuditLogRepository(db)),
		superAdmin.ID.String(),
	)
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}
	if converted == nil {
		t.Fatal("conversion returned no user")
	}

	// Returned model: same identity, new Admin role.
	if converted.ID != created.ID {
		t.Errorf("converted.ID = %v, want %v (user id must be preserved)", converted.ID, created.ID)
	}
	if converted.RoleID != adminRole.ID || converted.Role.Name != models.RoleAdmin {
		t.Errorf("converted role = %q (%v), want Admin (%v)", converted.Role.Name, converted.RoleID, adminRole.ID)
	}

	// Persisted row: only the role changed.
	reloaded := reloadSeedTestUser(t, db, created.ID)
	if reloaded.RoleID != adminRole.ID || reloaded.Role.Name != models.RoleAdmin {
		t.Errorf("persisted role = %q, want Admin", reloaded.Role.Name)
	}
	if reloaded.Username != baseline.Username {
		t.Errorf("username changed: %q -> %q", baseline.Username, reloaded.Username)
	}
	if reloaded.Email != baseline.Email {
		t.Errorf("email changed: %q -> %q", baseline.Email, reloaded.Email)
	}
	if reloaded.PasswordHash != baseline.PasswordHash {
		t.Error("password hash must be preserved by the conversion")
	}
	if reloaded.Status != baseline.Status {
		t.Errorf("status changed: %q -> %q", baseline.Status, reloaded.Status)
	}
	if !reloaded.CreatedAt.Equal(baseline.CreatedAt) {
		t.Errorf("created_at changed: %v -> %v", baseline.CreatedAt, reloaded.CreatedAt)
	}

	// Audit: exactly one record with actor, old role and new role.
	if got := conversionAuditCount(t, db, created.ID); got != 1 {
		t.Fatalf("conversion audit rows = %d, want 1", got)
	}
	var audit models.AuditLog
	if err := db.Where("action = ? AND entity_id = ?", "PARTNER_TO_ADMIN_ROLE_CHANGE", created.ID).
		First(&audit).Error; err != nil {
		t.Fatalf("failed to load conversion audit row: %v", err)
	}
	if audit.UserID == nil || *audit.UserID != superAdmin.ID {
		t.Errorf("audit actor = %v, want super admin %v", audit.UserID, superAdmin.ID)
	}
	if audit.OldValue != models.RolePartner {
		t.Errorf("audit old_value = %q, want %q", audit.OldValue, models.RolePartner)
	}
	if audit.NewValue != models.RoleAdmin {
		t.Errorf("audit new_value = %q, want %q", audit.NewValue, models.RoleAdmin)
	}
	if strings.Contains(strings.ToLower(audit.Details), "password") {
		t.Errorf("audit details must never mention credentials: %q", audit.Details)
	}

	// After the conversion, the intended account is now Admin and the
	// Partner role is free. Create a second Manager (Partner role) account
	// to verify it can be created after the first is converted, and that
	// it is not affected by the conversion.
	otherEmail := "ptoa-other-" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@pwams.local"
	otherPartner := seedTestUser(t, db, partnerRole.ID, otherEmail)
	other := reloadSeedTestUser(t, db, otherPartner.ID)
	if other.RoleID != partnerRole.ID || other.Role.Name != models.RolePartner {
		t.Errorf("other manager role = %q, want Partner (must remain untouched by the conversion)", other.Role.Name)
	}
	if got := conversionAuditCount(t, db, otherPartner.ID); got != 0 {
		t.Errorf("other manager has %d conversion audit rows, want 0", got)
	}
}

// TestIntegration_PartnerToAdminConversionIsIdempotent proves that re-running
// the conversion (e.g. every server start) reuses the already-converted
// account instead of creating duplicates or extra audit rows.
func TestIntegration_PartnerToAdminConversionIsIdempotent(t *testing.T) {
	db, cfg := partnerToAdminTestDB(t)

	adminRole := seedTestRoleByName(t, db, models.RoleAdmin)
	partnerRole := seedTestRoleByName(t, db, models.RolePartner)

	email := "ptoa-idem-" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@pwams.local"
	created := seedTestUser(t, db, partnerRole.ID, email)

	seedCfg := *cfg
	seedCfg.PartnerToAdminEmail = email
	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))

	first, err := PromoteConfiguredPartnerToAdmin(db, &seedCfg, auditSvc, "")
	if err != nil {
		t.Fatalf("first conversion failed: %v", err)
	}
	if first == nil || first.ID != created.ID {
		t.Fatalf("first conversion returned %+v, want the created account", first)
	}

	second, err := PromoteConfiguredPartnerToAdmin(db, &seedCfg, auditSvc, "")
	if err != nil {
		t.Fatalf("second conversion failed: %v", err)
	}
	if second == nil || second.ID != created.ID {
		t.Fatalf("second conversion returned %+v, want the SAME account (no duplicates)", second)
	}

	var userCount int64
	if err := db.Model(&models.User{}).Where("LOWER(email) = ?", email).Count(&userCount).Error; err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("users with the configured email = %d, want 1 (idempotent seed)", userCount)
	}

	if got := conversionAuditCount(t, db, created.ID); got != 1 {
		t.Errorf("conversion audit rows after re-run = %d, want 1", got)
	}

	reloaded := reloadSeedTestUser(t, db, created.ID)
	if reloaded.RoleID != adminRole.ID {
		t.Errorf("role after re-run = %v, want Admin", reloaded.RoleID)
	}
}

// TestIntegration_PartnerToAdminRefusesNonPartner proves the conversion never
// re-roles an account that does not hold the Partner role (Manager) (a Super Admin,
// Staff, Donor etc. configured by mistake is refused, not converted).
func TestIntegration_PartnerToAdminRefusesNonPartner(t *testing.T) {
	db, cfg := partnerToAdminTestDB(t)

	staffRole := seedTestRoleByName(t, db, models.RoleStaff)
	adminRole := seedTestRoleByName(t, db, models.RoleAdmin)

	email := "ptoa-staff-" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@pwams.local"
	created := seedTestUser(t, db, staffRole.ID, email)

	seedCfg := *cfg
	seedCfg.PartnerToAdminEmail = email

	converted, err := PromoteConfiguredPartnerToAdmin(
		db,
		&seedCfg,
		services.NewAuditLogService(repository.NewAuditLogRepository(db)),
		"",
	)
	if !errors.Is(err, ErrPartnerToAdminNotPartner) {
		t.Fatalf("error = %v, want ErrPartnerToAdminNotPartner", err)
	}
	if converted != nil {
		t.Fatalf("conversion returned %+v, want nil", converted)
	}

	reloaded := reloadSeedTestUser(t, db, created.ID)
	if reloaded.RoleID != staffRole.ID || reloaded.RoleID == adminRole.ID {
		t.Errorf("staff account role changed to %q — non-Manager (Partner role) accounts must never convert", reloaded.Role.Name)
	}
	if got := conversionAuditCount(t, db, created.ID); got != 0 {
		t.Errorf("refused conversion wrote %d audit rows, want 0", got)
	}
}

// TestIntegration_PartnerToAdminMissingAccount proves that a configured
// identifier without a matching account reports the dedicated error instead
// of failing with a raw database error.
func TestIntegration_PartnerToAdminMissingAccount(t *testing.T) {
	db, cfg := partnerToAdminTestDB(t)

	seedCfg := *cfg
	seedCfg.PartnerToAdminEmail = "ptoa-missing-" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@pwams.local"

	converted, err := PromoteConfiguredPartnerToAdmin(
		db,
		&seedCfg,
		services.NewAuditLogService(repository.NewAuditLogRepository(db)),
		"",
	)
	if !errors.Is(err, ErrPartnerToAdminNotFound) {
		t.Fatalf("error = %v, want ErrPartnerToAdminNotFound", err)
	}
	if converted != nil {
		t.Fatalf("conversion returned %+v, want nil", converted)
	}
}

// TestIntegration_PartnerToAdminUnconfiguredIsNoOp proves the conversion is a
// no-op when PARTNER_TO_ADMIN_EMAIL is empty or blank — the server must start
// cleanly without touching any account.
func TestIntegration_PartnerToAdminUnconfiguredIsNoOp(t *testing.T) {
	db, cfg := partnerToAdminTestDB(t)

	for _, blank := range []string{"", "   "} {
		seedCfg := *cfg
		seedCfg.PartnerToAdminEmail = blank

		converted, err := PromoteConfiguredPartnerToAdmin(
			db,
			&seedCfg,
			services.NewAuditLogService(repository.NewAuditLogRepository(db)),
			"",
		)
		if err != nil {
			t.Fatalf("unconfigured conversion (identifier %q) failed: %v", blank, err)
		}
		if converted != nil {
			t.Fatalf("unconfigured conversion (identifier %q) returned %+v, want nil", blank, converted)
		}
	}
}
