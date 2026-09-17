package services_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

func step3TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("step3 test skipped (configuration unavailable): %v", err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("step3 test skipped (database unavailable): %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("step3 test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("step3 test role seed failed: %v", err)
	}
	return db
}

func TestStep3_NewUserIsActive(t *testing.T) {
	db := step3TestDB(t)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "new-active-" + suffix + "@pwams.local",
		FullName:        "New Active User",
		Role:            models.RoleStaff,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("user creation failed: %v", err)
	}
	if user.Status != models.UserStatusActive {
		t.Fatalf("newly created user status = %q, want %q", user.Status, models.UserStatusActive)
	}
}

func TestStep3_PendingUserStatusRejected(t *testing.T) {
	db := step3TestDB(t)
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
		Role:         staffRole,
		Status:       "Pending",
		PasswordHash: "$2a$10$dummyhashdummyhashdummyhashdummyhashdummyhashdummyhashdum",
	}
	err := userRepo.Create(user)
	if err == nil {
		t.Fatal("database must reject a user with status 'Pending'")
	}
	if !strings.Contains(err.Error(), "chk_users_status_no_pending") && !strings.Contains(err.Error(), "violates check constraint") {
		t.Fatalf("expected CHECK constraint violation, got: %v", err)
	}
}

func cleanupExistingManagers(t *testing.T, db *gorm.DB) {
	t.Helper()
	var partnerRole models.Role
	if err := db.Where("name = ?", models.RolePartner).First(&partnerRole).Error; err != nil {
		return
	}
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

func TestStep3_ManagerUniqueness_FirstManagerSucceeds(t *testing.T) {
	db := step3TestDB(t)
	cleanupExistingManagers(t, db)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "first-mgr-" + suffix + "@pwams.local",
		FullName:        "First Manager",
		Role:            models.RolePartner,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("first Manager creation must succeed, got: %v", err)
	}
}

func TestStep3_ManagerUniqueness_SecondManagerFails(t *testing.T) {
	db := step3TestDB(t)
	cleanupExistingManagers(t, db)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "mgr-one-" + suffix + "@pwams.local",
		FullName:        "Manager One",
		Role:            models.RolePartner,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("first Manager creation failed: %v", err)
	}

	_, err = svc.CreateUser(models.CreateUserRequest{
		Email:           "mgr-two-" + suffix + "@pwams.local",
		FullName:        "Manager Two",
		Role:            models.RolePartner,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err == nil {
		t.Fatal("second Manager creation must fail with ErrManagerAlreadyExists")
	}
	if !strings.Contains(err.Error(), "Only one Manager") {
		t.Fatalf("expected ErrManagerAlreadyExists, got: %v", err)
	}
}
