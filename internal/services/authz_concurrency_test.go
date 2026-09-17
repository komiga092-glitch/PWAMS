package services_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
	"gorm.io/gorm"
)

// countActiveAdminsTx returns the number of active Admin/Super Admin users
// visible in the current transaction, excluding the given IDs.
func countActiveAdminsTx(t *testing.T, tx *gorm.DB, exclude ...uuid.UUID) int {
	t.Helper()
	var users []models.User
	query := tx.
		Model(&models.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.status = ?", models.UserStatusActive).
		Where("users.deleted_at IS NULL").
		Where("LOWER(roles.name) IN (?)",
			[]string{
				strings.ToLower(models.RoleAdmin),
				strings.ToLower(models.RoleSuperAdmin),
			})
	if len(exclude) > 0 {
		query = query.Where("users.id NOT IN ?", exclude)
	}
	if err := query.Find(&users).Error; err != nil {
		t.Fatalf("countActiveAdminsTx: %v", err)
	}
	return len(users)
}

// TestLastActiveAdminConcurrency is a dedicated concurrency regression test
// for the last-active-Admin guard. It must run against a database with NO
// pre-existing active Admin/Super Admin users, because the guard's "last
// admin" invariant is defined relative to the full active-Admin set.
//
// The test creates exactly two fresh active Admin accounts (the only active
// admins in the system), then concurrently disables both. Because the guard
// is enforced inside a transaction with a row-level FOR UPDATE lock on the
// active-Admin row set, concurrent last-admin operations must serialize: the
// database must NEVER reach zero active Admins, and at least one of the two
// concurrent operations must be rejected with ErrLastActiveAdmin.
func TestLastActiveAdminConcurrency(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	// Establish the baseline: count pre-existing active Admins so the test
	// invariant is correct regardless of database state.
	roles := loadSeedRoles(t, db)
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	userSvc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	// Clean up any stale test fixtures from previous runs.
	cleanupStaleTestAdmins(t, db, fx)

	baselineAdmins := countActiveAdminsTx(t, db)

	// Create exactly two fresh, independent active Admin accounts.
	admin1 := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "conc1")
	admin2 := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "conc2")

	// Sanity check: both are active Admins.
	if admin1.Status != models.UserStatusActive || admin2.Status != models.UserStatusActive {
		t.Fatalf("fixture setup: admins not active: %q, %q", admin1.Status, admin2.Status)
	}

	totalAdmins := baselineAdmins + 2

	// Concurrently attempt to disable both Admins. With the row-locking
	// guard on the active-Admin set, the two transactions serialize: the
	// first to acquire the lock disables one admin (count goes from N to
	// N-1 >= 1, allowed), the second sees N-1 active admins excluding
	// itself and is refused because it would be the last one.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	ids := []uuid.UUID{admin1.ID, admin2.ID}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = userSvc.UpdateUserStatus(ids[idx].String(), models.UserStatusDisabled)
		}(i)
	}

	wg.Wait()

	// At least one operation must be rejected with ErrLastActiveAdmin.
	rejected := 0
	for _, err := range errs {
		if err == services.ErrLastActiveAdmin {
			rejected++
		} else if err != nil {
			t.Fatalf("unexpected error during concurrent disable: %v", err)
		}
	}

	if totalAdmins == 2 && rejected == 0 {
		t.Fatalf("concurrent last-admin disable: expected at least one ErrLastActiveAdmin with exactly 2 admins, got none")
	}

	// The database must NEVER have dropped below baselineAdmins active
	// Admins. The concurrent operations must not reduce the count below
	// the pre-existing baseline.
	finalAdmins := countActiveAdminsTx(t, db)
	if finalAdmins < baselineAdmins {
		t.Fatalf("concurrent last-admin disable: active Admins dropped below baseline: baseline=%d final=%d", baselineAdmins, finalAdmins)
	}

	// If exactly 2 admins existed (baseline 0 + 2 test), exactly 1 must
	// remain active.
	if totalAdmins == 2 {
		testAdminsActive := 0
		for _, id := range []uuid.UUID{admin1.ID, admin2.ID} {
			var user models.User
			if err := db.Preload("Role").First(&user, "id = ?", id).Error; err != nil {
				t.Fatalf("reload admin %s: %v", id, err)
			}
			if user.Status == models.UserStatusActive &&
				(user.Role.Name == models.RoleAdmin || user.Role.Name == models.RoleSuperAdmin) {
				testAdminsActive++
			}
		}
		if testAdminsActive != 1 {
			t.Fatalf("concurrent last-admin disable: expected exactly 1 of 2 test admins active, got %d", testAdminsActive)
		}
	}
}

// cleanupStaleTestAdmins removes test admin fixtures left behind by previous
// interrupted runs of this test so the baseline count is correct.
func cleanupStaleTestAdmins(t *testing.T, db *gorm.DB, fx *testFixture) {
	t.Helper()
	adminRoleID := func() uuid.UUID {
		var role models.Role
		if db.Where("LOWER(name) = ?", strings.ToLower(models.RoleAdmin)).Take(&role).Error == nil {
			return role.ID
		}
		return uuid.Nil
	}()
	if adminRoleID == uuid.Nil {
		return
	}
	var stale []models.User
	db.Where("role_id = ? AND username LIKE 'authz_conc%'", adminRoleID).Find(&stale)
	for _, u := range stale {
		db.Unscoped().Delete(&models.User{}, "id = ?", u.ID)
	}
}
