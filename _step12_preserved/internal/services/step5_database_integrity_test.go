package services_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// databaseIntegrityTestDB sets up a test database for data-integrity tests.
// Tests are skipped when no database is available.
func databaseIntegrityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("database integrity test skipped (configuration unavailable): %v", err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("database integrity test skipped (database unavailable): %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("database integrity test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("database integrity test role seed failed: %v", err)
	}
	permSvc := services.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewRoleRepository(db),
		repository.NewAuditLogRepository(db),
	)
	if err := permSvc.SeedDefaults(); err != nil {
		t.Fatalf("database integrity test permission seed failed: %v", err)
	}
	return db
}

// ---------------------------------------------------------------------------
// USER STATUS INTEGRITY
// ---------------------------------------------------------------------------

func TestIntegrity_UserStatus_ActiveAccepted(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "status-active-" + suffix + "@pwams.local",
		FullName:        "Active User",
		Role:            models.RoleStaff,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("creating active user failed: %v", err)
	}
	if user.Status != models.UserStatusActive {
		t.Fatalf("status = %q, want %q", user.Status, models.UserStatusActive)
	}
	// Verify persisted in DB
	var persisted models.User
	if err := db.First(&persisted, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if persisted.Status != models.UserStatusActive {
		t.Fatalf("persisted status = %q, want %q", persisted.Status, models.UserStatusActive)
	}
}

func TestIntegrity_UserStatus_PendingRejected(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	userRepo := repository.NewUserRepository(db)

	var staffRole models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&staffRole).Error; err != nil {
		t.Fatalf("resolve Staff role: %v", err)
	}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user := &models.User{
		Username:     "pending_" + suffix,
		Email:        "pending-" + suffix + "@pwams.local",
		FullName:     "Pending User",
		RoleID:       staffRole.ID,
		Status:       "Pending",
		PasswordHash: "$2a$10$dummyhashdummyhashdummyhashdummyhashdummyhashdummyhashdum",
	}
	err := userRepo.Create(user)
	if err == nil {
		t.Fatal("database must reject a user with status 'Pending'")
	}
	if !strings.Contains(err.Error(), "chk_users_status_no_pending") &&
		!strings.Contains(err.Error(), "violates check constraint") {
		t.Fatalf("expected CHECK constraint violation, got: %v", err)
	}
}

func TestIntegrity_UserStatus_DisabledAccepted(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "status-disabled-" + suffix + "@pwams.local",
		FullName:        "Disabled User",
		Role:            models.RoleStaff,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("creating user failed: %v", err)
	}

	// Directly set status to verify database accepts it
	if err := db.Model(&user).Update("status", models.UserStatusDisabled).Error; err != nil {
		t.Fatalf("updating to Disabled failed: %v", err)
	}

	var persisted models.User
	if err := db.First(&persisted, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if persisted.Status != models.UserStatusDisabled {
		t.Fatalf("status = %q, want %q", persisted.Status, models.UserStatusDisabled)
	}
}

func TestIntegrity_UserStatus_LockedAccepted(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "status-locked-" + suffix + "@pwams.local",
		FullName:        "Locked User",
		Role:            models.RoleStaff,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("creating user failed: %v", err)
	}

	if err := db.Model(&user).Update("status", models.UserStatusLocked).Error; err != nil {
		t.Fatalf("updating to Locked failed: %v", err)
	}

	var persisted models.User
	if err := db.First(&persisted, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if persisted.Status != models.UserStatusLocked {
		t.Fatalf("status = %q, want %q", persisted.Status, models.UserStatusLocked)
	}
}

// ---------------------------------------------------------------------------
// ROLE INTEGRITY
// ---------------------------------------------------------------------------

func TestIntegrity_DuplicateRoleRejected(t *testing.T) {
	db := databaseIntegrityTestDB(t)

	role := models.Role{
		Name:        models.RoleStaff,
		Description: "Duplicate role",
	}
	err := db.Create(&role).Error
	if err == nil {
		t.Fatal("database must reject duplicate role name")
	}
	if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected unique constraint violation, got: %v", err)
	}
}

func TestIntegrity_ProtectedRoleCannotBeDeleted(t *testing.T) {
	db := databaseIntegrityTestDB(t)

	var superAdminRole models.Role
	if err := db.Where("name = ?", models.RoleSuperAdmin).First(&superAdminRole).Error; err != nil {
		t.Fatalf("resolve Super Admin role: %v", err)
	}

	// Create a user that references the Super Admin role so the FK constraint
	// (ON DELETE RESTRICT) is exercised.
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	testUser := &models.User{
		Username:     "prot-role-" + suffix,
		Email:        "protected-role-test-" + suffix + "@pwams.local",
		FullName:     "Protected Role Test User",
		RoleID:       superAdminRole.ID,
		Status:       models.UserStatusActive,
		PasswordHash: "$2a$10$dummyhash",
	}
	if err := db.Create(testUser).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer db.Unscoped().Delete(&models.User{}, "id = ?", testUser.ID)

	// Attempt a hard delete (Unscoped). The users table has ON DELETE RESTRICT,
	// so if a user references this role, the delete must be rejected by the FK.
	err := db.Unscoped().Delete(&models.Role{}, "id = ?", superAdminRole.ID).Error
	if err == nil {
		t.Fatal("database must reject hard-deleting a role that has users (FK RESTRICT)")
	}
}

// ---------------------------------------------------------------------------
// MANAGER UNIQUENESS
// ---------------------------------------------------------------------------

func TestIntegrity_ManagerUniqueness_DatabaseEnforced(t *testing.T) {
	db := databaseIntegrityTestDB(t)

	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	var partnerRole models.Role
	if err := db.Where("name = ?", models.RolePartner).First(&partnerRole).Error; err == nil {
		db.Unscoped().Where("role_id = ?", partnerRole.ID).Delete(&models.User{})
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")

	// First Manager -> PASS
	_, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "mgr-db-first-" + suffix + "@pwams.local",
		FullName:        "DB First Manager",
		Role:            models.RolePartner,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("first Manager creation must succeed, got: %v", err)
	}

	// Second active Manager -> REJECT (service-level)
	_, err = svc.CreateUser(models.CreateUserRequest{
		Email:           "mgr-db-second-" + suffix + "@pwams.local",
		FullName:        "DB Second Manager",
		Role:            models.RolePartner,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err == nil {
		t.Fatal("second Manager creation must fail")
	}
	if !strings.Contains(err.Error(), "Only one Manager") {
		t.Fatalf("expected ErrManagerAlreadyExists, got: %v", err)
	}

	// Also verify database-level enforcement
	secondMgr := &models.User{
		Username:     "mgr-db-direct-" + suffix,
		Email:        "mgr-db-direct-" + suffix + "@pwams.local",
		FullName:     "DB Direct Manager",
		RoleID:       partnerRole.ID,
		Status:       models.UserStatusActive,
		PasswordHash: "$2a$10$dummyhash",
	}
	err = db.Create(secondMgr).Error
	if err == nil {
		t.Fatal("database must reject second active Manager via partial unique index")
	}
}

func TestIntegrity_ManagerUniqueness_ConcurrentCreation(t *testing.T) {
	db := databaseIntegrityTestDB(t)

	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	// Clean slate: remove any existing Partner (Manager) accounts including
	// related referencing rows, so the concurrent test starts from a state
	// where zero Managers exist.
	var partnerRole models.Role
	if err := db.Where("name = ?", models.RolePartner).First(&partnerRole).Error; err == nil {
		var partners []models.User
		db.Unscoped().Where("role_id = ?", partnerRole.ID).Find(&partners)
		for _, u := range partners {
			_ = db.Where("user_id = ?", u.ID).Delete(&models.Session{}).Error
			var sessions []models.Session
			db.Unscoped().Where("user_id = ?", u.ID).Find(&sessions)
			_ = db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.Session{}).Error
			_ = db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.PasswordResetToken{}).Error
		}
		db.Unscoped().Where("role_id = ?", partnerRole.ID).Delete(&models.User{})
	}

	// Fire N concurrent creation requests. The service-level check races, but
	// the partial unique index uq_users_one_manager guarantees at most one
	// Partner user can be inserted with deleted_at IS NULL.
	const n = 8
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			_, err := svc.CreateUser(models.CreateUserRequest{
				Email:           fmt.Sprintf("mgr-race-%d-%s@pwams.local", idx, suffix),
				FullName:        "Race Manager",
				Role:            models.RolePartner,
				Password:        "TestPassword123!",
				ConfirmPassword: "TestPassword123!",
			})
			results <- err
		}(i)
	}

	successes := 0
	for i := 0; i < n; i++ {
		err := <-results
		if err == nil {
			successes++
		}
	}
	close(results)

	if successes != 1 {
		t.Fatalf("concurrent Manager creation: successes = %d, want exactly 1", successes)
	}

	// The database must hold exactly one active Manager regardless of what the
	// service layer allowed through.
	var mgrCount int64
	if err := db.Model(&models.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("LOWER(roles.name) = LOWER(?)", models.RolePartner).
		Count(&mgrCount).Error; err != nil {
		t.Fatalf("count remaining Managers: %v", err)
	}
	if mgrCount != 1 {
		t.Fatalf("active Managers in database = %d, want exactly 1", mgrCount)
	}
}

// ---------------------------------------------------------------------------
// FINANCIAL DATA INTEGRITY
// ---------------------------------------------------------------------------

func TestIntegrity_CareProvidedAmountIsDecimal(t *testing.T) {
	var cp models.CareProvided
	var _ decimal.Decimal = cp.Amount
}

func TestIntegrity_LoanAmountIsDecimal(t *testing.T) {
	var loan models.Loan
	var _ decimal.Decimal = loan.LoanAmount
	var _ decimal.Decimal = loan.InstallmentAmount
}

// ---------------------------------------------------------------------------
// SOFT DELETE INTEGRITY
// ---------------------------------------------------------------------------

func TestIntegrity_SoftDeleteSetsIsDeleted(t *testing.T) {
	db := databaseIntegrityTestDB(t)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "softdel-" + suffix + "@pwams.local",
		FullName:        "Soft Delete User",
		Role:            models.RoleStaff,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("creating user failed: %v", err)
	}

	if err := db.Delete(&models.User{}, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}

	var count int64
	if err := db.Model(&models.User{}).Where("id = ?", user.ID).Count(&count).Error; err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("soft-deleted user should not appear in normal queries, count=%d", count)
	}

	var persisted models.User
	if err := db.Unscoped().First(&persisted, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("unscoped query failed: %v", err)
	}
	if !persisted.DeletedAt.Valid {
		t.Fatal("deleted_at should be set after soft delete")
	}
}

func TestIntegrity_SoftDeleteColumnsExist(t *testing.T) {
	db := databaseIntegrityTestDB(t)

	// These tables historically lacked a deleted_at column (or lacked the
	// model field), causing GORM to perform HARD deletes that permanently
	// removed records and broke sync propagation. Migration 000008 + the
	// runtime migration now guarantee the column exists.
	for _, table := range []string{"care_provided", "loans", "loan_repayments"} {
		if !db.Migrator().HasColumn(table, "deleted_at") {
			t.Errorf("table %s must have a deleted_at column (soft delete)", table)
		}
	}

	// The FK constraints added by migration 000008 / runtime migration must
	// exist at the database level.
	expectedFKs := map[string]string{
		"care_provided":   "fk_care_provided_aid_request",
		"loans":           "fk_loans_person",
		"loan_repayments": "fk_loan_repayments_loan",
	}
	var constraints []struct {
		Conname string
	}
	for table, fkName := range expectedFKs {
		constraints = constraints[:0]
		if err := db.Raw(`
			SELECT conname FROM pg_constraint
			WHERE conrelid = ?::regclass::oid AND contype = 'f'
		`, table).Scan(&constraints).Error; err != nil {
			t.Fatalf("query FK constraints for %s: %v", table, err)
		}
		found := false
		for _, c := range constraints {
			if c.Conname == fkName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("table %s must have FK constraint %q", table, fkName)
		}
	}
}

// ---------------------------------------------------------------------------
// FOREIGN KEY INTEGRITY
// ---------------------------------------------------------------------------

func TestIntegrity_ForeignKeyPreventsOrphanUser(t *testing.T) {
	db := databaseIntegrityTestDB(t)

	user := &models.User{
		Username:     "fk-test-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Email:        "fk-test-" + uuid.NewString() + "@pwams.local",
		FullName:     "FK Test",
		RoleID:       uuid.New(), // non-existent role
		Status:       models.UserStatusActive,
		PasswordHash: "$2a$10$dummyhash",
	}
	err := db.Create(user).Error
	if err == nil {
		t.Fatal("database must reject user with non-existent role_id")
	}
	if !strings.Contains(err.Error(), "foreign key") && !strings.Contains(err.Error(), "FK") {
		t.Fatalf("expected foreign key violation, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SEED SAFETY / IDEMPOTENCY
// ---------------------------------------------------------------------------

func TestIntegrity_SeedRolesIsIdempotent(t *testing.T) {
	db := databaseIntegrityTestDB(t)

	for i := 0; i < 3; i++ {
		if err := database.SeedDefaultRoles(db); err != nil {
			t.Fatalf("seed run %d failed: %v", i, err)
		}
	}

	for _, roleName := range models.AllRoles() {
		var count int64
		if err := db.Model(&models.Role{}).Where("name = ?", roleName).Count(&count).Error; err != nil {
			t.Fatalf("count role %s failed: %v", roleName, err)
		}
		if count != 1 {
			t.Errorf("role %q count = %d, want 1 (idempotent seed)", roleName, count)
		}
	}
}
