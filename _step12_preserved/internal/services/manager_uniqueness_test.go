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

func managerUniquenessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("manager uniqueness test skipped (configuration unavailable): %v", err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("manager uniqueness test skipped (database unavailable): %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("manager uniqueness test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("manager uniqueness test role seed failed: %v", err)
	}

	// Clean up any existing Partner (Manager) accounts from previous test runs
	// so each test starts from a known state. Delete related records first to
	// respect foreign key constraints.
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

	return db
}

func TestManagerUniqueness_FirstManagerSucceeds(t *testing.T) {
	db := managerUniquenessTestDB(t)
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

func TestManagerUniqueness_SecondManagerFails(t *testing.T) {
	db := managerUniquenessTestDB(t)
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

func TestManagerUniqueness_AfterSoftDeleteReplacementSucceeds(t *testing.T) {
	db := managerUniquenessTestDB(t)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	first, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "mgr-orig-" + suffix + "@pwams.local",
		FullName:        "Original Manager",
		Role:            models.RolePartner,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("first Manager creation failed: %v", err)
	}

	if err := db.Delete(first).Error; err != nil {
		t.Fatalf("soft-delete failed: %v", err)
	}

	replacement, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "mgr-repl-" + suffix + "@pwams.local",
		FullName:        "Replacement Manager",
		Role:            models.RolePartner,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("replacement Manager after soft-delete must succeed, got: %v", err)
	}
	if replacement == nil {
		t.Fatal("replacement Manager is nil")
	}
}
