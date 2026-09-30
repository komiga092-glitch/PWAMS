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

// Phase 2 regression tests: platform-level Super Admin protection and the
// last-active guards, enforced at the SERVICE layer (never UI-only).
//
//   - a non-Super-Admin actor can never create, modify, deactivate, delete or
//     re-password a Super Admin account
//   - a non-Super-Admin actor can never GRANT the Super Admin role (privilege
//     escalation through the generic users route)
//   - the last active Super Admin can never be disabled, deleted, or have its
//     Super Admin role removed
//   - the last active protected Admin can never be disabled
//   - an unknown actor role set fails closed on platform operations

func newUserServiceForTest(db *gorm.DB) *services.UserService {
	return services.NewUserService(
		repository.NewUserRepository(db),
		repository.NewRoleRepository(db),
		repository.NewSessionRepository(db),
	)
}

func setUserStatus(t *testing.T, db *gorm.DB, id uuid.UUID, status string) {
	t.Helper()

	if err := db.Model(&models.User{}).Where("id = ?", id).
		Update("status", status).Error; err != nil {
		t.Fatalf("failed to set status of %s: %v", id, err)
	}
}

// isolateProtectedAdminSet temporarily removes every OTHER active Admin/Super
// Admin from the protected set so the given user becomes the last active
// protected admin. The original statuses are restored when the test ends.
// This mirrors the existing Manager-slot tests, which likewise free the slot
// for the duration of one test.
func isolateProtectedAdminSet(t *testing.T, db *gorm.DB, keepID uuid.UUID) {
	t.Helper()

	var others []models.User
	if err := db.
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.status = ?", models.UserStatusActive).
		Where("LOWER(roles.name) IN ?", []string{"admin", "super admin"}).
		Where("users.id <> ?", keepID).
		Find(&others).Error; err != nil {
		t.Fatalf("failed to load active protected admins: %v", err)
	}

	for _, user := range others {
		setUserStatus(t, db, user.ID, models.UserStatusDisabled)
	}

	t.Cleanup(func() {
		for _, user := range others {
			_ = db.Model(&models.User{}).Where("id = ?", user.ID).
				Update("status", models.UserStatusActive).Error
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Super Admin creation
// ─────────────────────────────────────────────────────────────────────────────

func TestCreateUser_NonSuperAdminCannotCreateSuperAdmin(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	svc := newUserServiceForTest(db)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]

	// Admin, Manager, Staff actors and an unknown (empty) actor role set must
	// all be refused: only a Super Admin actor may mint a Super Admin.
	for _, actorRoles := range [][]string{
		{models.RoleAdmin},
		{models.RoleManager},
		{models.RoleStaff},
		{},
	} {
		_, err := svc.CreateUser(models.CreateUserRequest{
			Username: "saesc-" + suffix,
			Email:    "saesc-" + suffix + "@pwams.test",
			Role:     models.RoleSuperAdmin,
			Password: "TestPass123!",
		}, actorRoles...)

		if !errors.Is(err, services.ErrCannotModifySuperAdmin) {
			t.Fatalf("actor %v creating a Super Admin: err = %v, want ErrCannotModifySuperAdmin", actorRoles, err)
		}
	}
}

func TestCreateUser_SuperAdminCanCreateSuperAdmin(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	svc := newUserServiceForTest(db)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]

	created, err := svc.CreateUser(models.CreateUserRequest{
		Username: "saok-" + suffix,
		Email:    "saok-" + suffix + "@pwams.test",
		Role:     models.RoleSuperAdmin,
		Password: "TestPass123!",
	}, models.RoleSuperAdmin)
	if err != nil {
		t.Fatalf("Super Admin creating a Super Admin must be allowed, got: %v", err)
	}

	fx.addUser(created.ID)

	if created.Role.Name != models.RoleSuperAdmin {
		t.Fatalf("created role = %q, want %q", created.Role.Name, models.RoleSuperAdmin)
	}
}

func TestCreateUser_NonSuperAdminCanStillCreateOperationalRoles(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	svc := newUserServiceForTest(db)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]

	// The Super Admin guard must not over-block: an Admin keeps the
	// operational user-administration capability the SRS grants it.
	for _, roleName := range []string{models.RoleStaff, models.RoleVolunteer, models.RoleStudent} {
		created, err := svc.CreateUser(models.CreateUserRequest{
			Username: "op-" + strings.ReplaceAll(roleName, " ", "") + "-" + suffix,
			Email:    "op-" + strings.ReplaceAll(roleName, " ", "") + "-" + suffix + "@pwams.test",
			Role:     roleName,
			Password: "TestPass123!",
		}, models.RoleAdmin)
		if err != nil {
			t.Fatalf("Admin creating %s must be allowed, got: %v", roleName, err)
		}

		fx.addUser(created.ID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Role escalation through UpdateUser (granting Super Admin)
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateUser_NonSuperAdminCannotGrantSuperAdminRole(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "grantee")

	_, err := svc.UpdateUser(target.ID.String(), models.UpdateUserRequest{
		Username: target.Username,
		Email:    target.Email,
		Role:     models.RoleSuperAdmin,
		Status:   models.UserStatusActive,
	}, models.RoleAdmin)
	if !errors.Is(err, services.ErrCannotModifySuperAdmin) {
		t.Fatalf("Admin granting the Super Admin role: err = %v, want ErrCannotModifySuperAdmin", err)
	}

	// The privilege escalation must not have happened in the database.
	var reloaded models.User
	if err := db.Preload("Role").Where("id = ?", target.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload target user: %v", err)
	}
	if reloaded.Role.Name != models.RoleStaff {
		t.Fatalf("target role = %q after refused escalation, want %q", reloaded.Role.Name, models.RoleStaff)
	}
}

func TestUpdateUser_SuperAdminCanPromoteAnotherAccount(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "promotee")

	updated, err := svc.UpdateUser(target.ID.String(), models.UpdateUserRequest{
		Username: target.Username,
		Email:    target.Email,
		Role:     models.RoleSuperAdmin,
		Status:   models.UserStatusActive,
	}, models.RoleSuperAdmin)
	if err != nil {
		t.Fatalf("Super Admin promoting another account must be allowed, got: %v", err)
	}

	if updated.Role.Name != models.RoleSuperAdmin {
		t.Fatalf("updated role = %q, want %q", updated.Role.Name, models.RoleSuperAdmin)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Modifying an existing Super Admin account
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateUser_NonSuperAdminCannotModifySuperAdminAccount(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "target")

	_, err := svc.UpdateUser(target.ID.String(), models.UpdateUserRequest{
		Username: target.Username,
		Email:    target.Email,
		Role:     models.RoleSuperAdmin,
		Status:   models.UserStatusActive,
	}, models.RoleAdmin)
	if !errors.Is(err, services.ErrCannotModifySuperAdmin) {
		t.Fatalf("Admin modifying a Super Admin: err = %v, want ErrCannotModifySuperAdmin", err)
	}
}

func TestUpdateUserStatus_NonSuperAdminCannotDeactivateSuperAdmin(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "sa-status")

	err := svc.UpdateUserStatus(target.ID.String(), models.UserStatusDisabled, models.RoleAdmin)
	if !errors.Is(err, services.ErrCannotModifySuperAdmin) {
		t.Fatalf("Admin deactivating a Super Admin: err = %v, want ErrCannotModifySuperAdmin", err)
	}

	var reloaded models.User
	if err := db.Where("id = ?", target.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload target: %v", err)
	}
	if reloaded.Status != models.UserStatusActive {
		t.Fatalf("target status = %q, want it to remain %q", reloaded.Status, models.UserStatusActive)
	}
}

func TestResetPassword_NonSuperAdminCannotResetSuperAdminPassword(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "sa-pw")

	err := svc.ResetPassword(target.ID.String(), "NewPass123!", models.RoleAdmin)
	if !errors.Is(err, services.ErrCannotModifySuperAdmin) {
		t.Fatalf("Admin resetting a Super Admin password: err = %v, want ErrCannotModifySuperAdmin", err)
	}

	// A Super Admin actor keeps the capability for ordinary accounts.
	staff := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "pw-staff")
	if err := svc.ResetPassword(staff.ID.String(), "NewPass123!", models.RoleSuperAdmin); err != nil {
		t.Fatalf("Super Admin resetting a Staff password must be allowed, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Last active Super Admin / protected Admin
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateUserStatus_LastActiveSuperAdminCannotBeDisabled(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "lastsa")
	isolateProtectedAdminSet(t, db, target.ID)

	err := svc.UpdateUserStatus(target.ID.String(), models.UserStatusDisabled, models.RoleSuperAdmin)
	if !errors.Is(err, services.ErrLastActiveSuperAdmin) {
		t.Fatalf("disabling the last active Super Admin: err = %v, want ErrLastActiveSuperAdmin", err)
	}

	var reloaded models.User
	if err := db.Where("id = ?", target.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload target: %v", err)
	}
	if reloaded.Status != models.UserStatusActive {
		t.Fatalf("last active Super Admin status = %q, want it to remain %q", reloaded.Status, models.UserStatusActive)
	}
}

func TestUpdateUserStatus_LastActiveAdminCannotBeDisabled(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "lastadmin")
	isolateProtectedAdminSet(t, db, target.ID)

	// A Super Admin actor is required, otherwise the platform guard fires
	// first. The last-active guard must still refuse the operation.
	err := svc.UpdateUserStatus(target.ID.String(), models.UserStatusDisabled, models.RoleSuperAdmin)
	if !errors.Is(err, services.ErrLastActiveAdmin) {
		t.Fatalf("disabling the last active Admin: err = %v, want ErrLastActiveAdmin", err)
	}
}

func TestUpdateUser_LastActiveSuperAdminCannotLoseSuperAdminRole(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "demote-sa")
	isolateProtectedAdminSet(t, db, target.ID)

	_, err := svc.UpdateUser(target.ID.String(), models.UpdateUserRequest{
		Username: target.Username,
		Email:    target.Email,
		Role:     models.RoleStaff,
		Status:   models.UserStatusActive,
	}, models.RoleSuperAdmin)
	if !errors.Is(err, services.ErrLastActiveSuperAdmin) {
		t.Fatalf("removing the Super Admin role from the last active Super Admin: err = %v, want ErrLastActiveSuperAdmin", err)
	}

	var reloaded models.User
	if err := db.Preload("Role").Where("id = ?", target.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload target: %v", err)
	}
	if reloaded.Role.Name != models.RoleSuperAdmin {
		t.Fatalf("target role = %q after refused demotion, want %q", reloaded.Role.Name, models.RoleSuperAdmin)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Deletion safeguards (Super Admin protection + two-person Admin approval)
// ─────────────────────────────────────────────────────────────────────────────

func TestDeleteUser_LastActiveSuperAdminCannotBeDeleted(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "del-sa")

	// The actor must hold the platform permission (Super Admin) but must NOT
	// be part of the active protected set, otherwise the target would not be
	// the last one and the guard could not be observed.
	actor := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "del-sa-actor")
	setUserStatus(t, db, actor.ID, models.UserStatusDisabled)

	isolateProtectedAdminSet(t, db, target.ID)

	err := svc.DeleteUser(target.ID.String(), actor.ID.String(), models.RoleSuperAdmin)
	if !errors.Is(err, services.ErrLastActiveSuperAdmin) {
		t.Fatalf("deleting the last active Super Admin: err = %v, want ErrLastActiveSuperAdmin", err)
	}

	var reloaded models.User
	if err := db.Where("id = ?", target.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("last active Super Admin must still exist: %v", err)
	}
}

func TestDeleteUser_NonSuperAdminCannotDeleteSuperAdmin(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "del-sa2")
	actor := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "del-actor")

	err := svc.DeleteUser(target.ID.String(), actor.ID.String(), models.RoleAdmin)
	if !errors.Is(err, services.ErrCannotModifySuperAdmin) {
		t.Fatalf("Admin deleting a Super Admin: err = %v, want ErrCannotModifySuperAdmin", err)
	}
}

func TestDeleteUser_NonSuperAdminDeletingAdminRequiresApproval(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin-target")
	actor := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin-actor")

	err := svc.DeleteUser(target.ID.String(), actor.ID.String(), models.RoleAdmin)
	if !errors.Is(err, services.ErrAdminDeletionRequiresApproval) {
		t.Fatalf("Admin deleting another Admin via the generic route: err = %v, want ErrAdminDeletionRequiresApproval", err)
	}

	var reloaded models.User
	if err := db.Where("id = ?", target.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("target Admin must still exist: %v", err)
	}
}

func TestDeleteUser_SuperAdminCanDeleteAdminWhenAnotherRemains(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	svc := newUserServiceForTest(db)

	target := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "del-admin-ok")
	actor := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "del-admin-actor")

	if err := svc.DeleteUser(target.ID.String(), actor.ID.String(), models.RoleSuperAdmin); err != nil {
		t.Fatalf("Super Admin deleting an Admin while another protected admin remains must be allowed, got: %v", err)
	}

	var reloaded models.User
	if err := db.Where("id = ?", target.ID).First(&reloaded).Error; err == nil {
		t.Fatal("target Admin should have been soft-deleted")
	}
}
