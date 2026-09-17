package handlers

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestIntegration_LastActiveAdminProtection verifies the invariant that the
// system must never reach zero active Admin accounts through the normal Admin
// deletion/deactivation workflow.
func TestIntegration_LastActiveAdminProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	superAdmin := seededSuperAdmin(t, db)

	// Start from a clean slate: these assertions are sensitive to the absolute
	// active-admin count, so every pre-existing Admin account must be removed
	// first.
	cleanupAllAdmins(t, db)

	admin := createActiveAdmin(t, db, "int-last-1@pwams.local")

	// 1 active Admin -> cannot remove that Admin.
	err := deleteUserAs(t, db, superAdmin, admin.ID.String())
	if !errors.Is(err, services.ErrLastActiveAdmin) {
		t.Fatalf("deleting the only active Admin must fail with ErrLastActiveAdmin, got %v", err)
	}

	// 2 active Admins -> one can be removed.
	admin2 := createActiveAdmin(t, db, "int-last-2@pwams.local")
	if err := deleteUserAs(t, db, superAdmin, admin2.ID.String()); err != nil {
		t.Fatalf("removing one of two active Admins must succeed: %v", err)
	}

	// After removal leaves 1 -> remaining Admin cannot be removed.
	err = deleteUserAs(t, db, superAdmin, admin.ID.String())
	if !errors.Is(err, services.ErrLastActiveAdmin) {
		t.Fatalf("deleting the last remaining active Admin must fail with ErrLastActiveAdmin, got %v", err)
	}
}

// TestIntegration_LastActiveAdminDeactivationBlocked verifies that
// deactivating the last active Admin is blocked through the status-change
// path (UpdateUserStatus), not only the deletion path.
func TestIntegration_LastActiveAdminDeactivationBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	superAdmin := seededSuperAdmin(t, db)

	// Start from a clean slate: these assertions are sensitive to the absolute
	// active-admin count, so every pre-existing Admin account must be removed
	// first.
	cleanupAllAdmins(t, db)

	admin := createActiveAdmin(t, db, "int-deact-1@pwams.local")

	// Deactivating the only active Admin must be blocked.
	err := updateStatusAs(t, db, superAdmin, admin.ID.String(), models.UserStatusDisabled)
	if !errors.Is(err, services.ErrLastActiveAdmin) {
		t.Fatalf("deactivating the only active Admin must fail with ErrLastActiveAdmin, got %v", err)
	}
}

// TestIntegration_LastActiveAdminConcurrentDeletion verifies that two
// simultaneous deletion approvals cannot both pass a stale active-admin
// count and result in zero active Admins. The admin-management advisory lock
// serialises the operations so the second one observes the post-deletion
// count and is rejected.
func TestIntegration_LastActiveAdminConcurrentDeletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	superAdmin := seededSuperAdmin(t, db)

	// Start from a clean slate: these assertions are sensitive to the absolute
	// active-admin count, so every pre-existing Admin account must be removed
	// first.
	cleanupAllAdmins(t, db)

	adminA := createActiveAdmin(t, db, "int-conc-a@pwams.local")
	adminB := createActiveAdmin(t, db, "int-conc-b@pwams.local")

	// Fire both deletions concurrently. Exactly one must succeed and the
	// other must be rejected with ErrLastActiveAdmin — the system must not
	// reach zero active Admins.
	var wg sync.WaitGroup
	results := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0] = deleteUserAs(t, db, superAdmin, adminA.ID.String())
	}()
	go func() {
		defer wg.Done()
		results[1] = deleteUserAs(t, db, superAdmin, adminB.ID.String())
	}()
	wg.Wait()

	successCount := 0
	var lastErr error
	for _, err := range results {
		if err == nil {
			successCount++
		} else {
			lastErr = err
		}
	}

	if successCount != 1 {
		t.Fatalf("exactly one concurrent deletion must succeed, got %d (last error: %v)", successCount, lastErr)
	}
	if !errors.Is(lastErr, services.ErrLastActiveAdmin) {
		t.Fatalf("the rejected concurrent deletion must fail with ErrLastActiveAdmin, got %v", lastErr)
	}
}

// ---- helpers ----

func seededSuperAdmin(t *testing.T, db *gorm.DB) *models.User {
	t.Helper()

	var superAdmin models.User
	if err := db.
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("LOWER(roles.name) = ?", strings.ToLower(models.RoleSuperAdmin)).
		First(&superAdmin).Error; err != nil {
		t.Fatalf("cannot load seeded Super Admin: %v", err)
	}
	superAdmin.Role = models.Role{Name: models.RoleSuperAdmin}
	return &superAdmin
}

func createActiveAdmin(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()

	handler := adminHandlerForTest(db)
	body := fmt.Sprintf(
		`{"email":%q,"full_name":"Admin","password_setup":"temporary_password","temporary_password":"TempPass123!"}`,
		email,
	)
	w := performJSON(
		adminCreateRouter(handler, models.RoleAdmin, "admin.create", "admin.view"),
		"POST", "/admins", body,
	)
	if w.Code != 201 {
		t.Fatalf("create admin %s status=%d body=%s", email, w.Code, w.Body.String())
	}
	resp := parseCreateAdminResponse(t, []byte(w.Body.String()))
	adminID, err := uuid.Parse(resp.User.ID)
	if err != nil {
		t.Fatalf("admin %s has non-UUID id %q: %v", email, resp.User.ID, err)
	}
	return &models.User{
		ID:       adminID,
		Role:     models.Role{Name: models.RoleAdmin},
		Username: "admin",
		Status:   models.UserStatusActive,
	}
}

func deleteUserAs(t *testing.T, db *gorm.DB, actor *models.User, targetID string) error {
	t.Helper()

	userSvc := services.NewUserService(
		repository.NewUserRepository(db),
		repository.NewRoleRepository(db),
		repository.NewSessionRepository(db),
	)
	return userSvc.DeleteUser(targetID, actor.ID.String(), actor.Role.Name)
}

func updateStatusAs(t *testing.T, db *gorm.DB, actor *models.User, targetID, status string) error {
	t.Helper()

	userSvc := services.NewUserService(
		repository.NewUserRepository(db),
		repository.NewRoleRepository(db),
		repository.NewSessionRepository(db),
	)
	return userSvc.UpdateUserStatus(targetID, status, actor.Role.Name)
}
