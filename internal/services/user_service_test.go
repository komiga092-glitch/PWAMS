package services_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestCreateUser_RejectsSecondActiveManager
// ─────────────────────────────────────────────────────────────────────────────

func TestCreateUser_RejectsSecondActiveManager(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()
	roles := loadSeedRoles(t, db)

	// Ensure exactly one active Manager exists (reuse or create).
	if existingManagerID(db) == uuid.Nil {
		_ = makeUser(t, db, fx, roles.ManagerID, models.RoleManager, "existing")
	}

	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	_, err := svc.CreateUser(models.CreateUserRequest{
		Username: "mgr2-" + suffix,
		Email:    "mgr2-" + suffix + "@pwams.test",
		Role:     models.RoleManager,
		Password: "TestPass123!",
	})
	if err == nil {
		t.Fatal("expected ErrActiveManagerExists, got nil")
	}
	if err != services.ErrActiveManagerExists {
		t.Fatalf("expected ErrActiveManagerExists, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCreateUser_AllowsManagerWhenNoneActive
// ─────────────────────────────────────────────────────────────────────────────

func TestCreateUser_AllowsManagerWhenNoneActive(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	// Disable any existing active Manager so the slot is free.
	existingID := existingManagerID(db)
	if existingID != uuid.Nil {
		if err := db.Model(&models.User{}).Where("id = ?", existingID).Update("status", models.UserStatusDisabled).Error; err != nil {
			t.Fatalf("failed to disable existing manager: %v", err)
		}
		t.Cleanup(func() {
			_ = db.Model(&models.User{}).Where("id = ?", existingID).Update("status", models.UserStatusActive).Error
		})
	}

	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	user, err := svc.CreateUser(models.CreateUserRequest{
		Username: "mgr-new-" + suffix,
		Email:    "mgr-new-" + suffix + "@pwams.test",
		Role:     models.RoleManager,
		Password: "TestPass123!",
	})
	if err != nil {
		t.Fatalf("expected manager creation to succeed, got: %v", err)
	}
	fx.addUser(user.ID)
	if user.Role.Name != models.RoleManager {
		t.Fatalf("expected role Manager, got %q", user.Role.Name)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestUpdateUserStatus_RejectsActivatingSecondManager
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateUserStatus_RejectsActivatingSecondManager(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)

	// Ensure one active manager exists.
	if existingManagerID(db) == uuid.Nil {
		_ = makeUser(t, db, fx, roles.ManagerID, models.RoleManager, "active")
	}

	// Create a second manager but immediately disable it.
	disabledMgr := makeUser(t, db, fx, roles.ManagerID, models.RoleManager, "disabled2nd")
	if err := db.Model(&models.User{}).Where("id = ?", disabledMgr.ID).Update("status", models.UserStatusDisabled).Error; err != nil {
		t.Fatalf("could not disable second manager: %v", err)
	}

	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	err := svc.UpdateUserStatus(disabledMgr.ID.String(), models.UserStatusActive)
	if err == nil {
		t.Fatal("expected ErrActiveManagerExists, got nil")
	}
	if err != services.ErrActiveManagerExists {
		t.Fatalf("expected ErrActiveManagerExists, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestUpdateUserStatus_AllowsManagerActivationAfterOtherDeactivated
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateUserStatus_AllowsManagerActivationAfterOtherDeactivated(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)

	// Disable any currently active Manager so the slot is free.
	prevID := existingManagerID(db)
	if prevID != uuid.Nil {
		if err := db.Model(&models.User{}).Where("id = ?", prevID).Update("status", models.UserStatusDisabled).Error; err != nil {
			t.Fatalf("could not disable existing manager: %v", err)
		}
		t.Cleanup(func() {
			_ = db.Model(&models.User{}).Where("id = ?", prevID).Update("status", models.UserStatusActive).Error
		})
	}

	// Create a new disabled Manager (insert directly to skip service check).
	candidateMgr := makeUser(t, db, fx, roles.ManagerID, models.RoleManager, "candidate")
	if err := db.Model(&models.User{}).Where("id = ?", candidateMgr.ID).Update("status", models.UserStatusDisabled).Error; err != nil {
		t.Fatalf("could not disable candidate manager: %v", err)
	}

	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	err := svc.UpdateUserStatus(candidateMgr.ID.String(), models.UserStatusActive)
	if err != nil {
		t.Fatalf("expected activation to succeed after other deactivated, got: %v", err)
	}
}
