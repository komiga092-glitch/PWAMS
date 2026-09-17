package services_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

// loginTestDB connects to the configured PostgreSQL database and brings it to
// the canonical state (migrations + role seed). When no database is reachable
// the tests are skipped so the suite still runs in a plain `go test ./...`
// environment; CI provides a database service.
func loginTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("auth-service login test skipped (configuration unavailable): %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("auth-service login test skipped (database unavailable): %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("auth-service login test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("auth-service login test role seed failed: %v", err)
	}

	return db
}

// seedLoginUser inserts an Active account with a real bcrypt password hash
// and registers cleanup so repeated runs never collide on the unique
// email/username indexes.
func seedLoginUser(t *testing.T, db *gorm.DB, password string) models.User {
	t.Helper()

	var role models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&role).Error; err != nil {
		t.Fatalf("cannot resolve Staff role: %v", err)
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		t.Fatalf("cannot hash password: %v", err)
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user := models.User{
		Username:     "logintest_" + suffix,
		Email:        "logintest_" + suffix + "@pwams.local",
		FullName:     "Login Test Account",
		PasswordHash: hash,
		RoleID:       role.ID,
		Status:       models.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create login test user: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Where("user_id = ?", user.ID).Delete(&models.AuditLog{}).Error
		_ = db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{}).Error
	})

	return user
}

// TestAuthServiceLogin_SuccessfulLoginDoesNotConsumeFailedAttemptBudget pins
// the login-reliability rule: signing in with correct credentials never
// increments the account's failed-login counter (rate limiting counts only
// FAILED authentication attempts).
func TestAuthServiceLogin_SuccessfulLoginDoesNotConsumeFailedAttemptBudget(t *testing.T) {
	db := loginTestDB(t)
	user := seedLoginUser(t, db, "correct-password-1")

	auth := services.NewAuthService(repository.NewUserRepository(db))

	loggedIn, err := auth.Login(user.Email, "correct-password-1")
	if err != nil {
		t.Fatalf("successful login failed: %v", err)
	}
	if loggedIn.ID != user.ID {
		t.Fatalf("login returned user %s, want %s", loggedIn.ID, user.ID)
	}

	var reloaded models.User
	if err := db.Where("id = ?", user.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("cannot reload user: %v", err)
	}
	if reloaded.FailedLoginAttempts != 0 {
		t.Errorf("FailedLoginAttempts = %d after successful login, want 0", reloaded.FailedLoginAttempts)
	}
	if reloaded.LastLoginAt == nil {
		t.Error("LastLoginAt was not set on successful login")
	}
}

// TestAuthServiceLogin_ThreeFailedAttemptsLockAccount pins the existing
// account-lock behaviour: the third failed attempt locks the account for the
// lockout duration and further attempts (even with the correct password) are
// rejected while locked.
func TestAuthServiceLogin_ThreeFailedAttemptsLockAccount(t *testing.T) {
	db := loginTestDB(t)
	user := seedLoginUser(t, db, "correct-password-2")

	auth := services.NewAuthService(repository.NewUserRepository(db))

	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := auth.Login(user.Email, "wrong-password"); !errors.Is(err, services.ErrInvalidCredentials) {
			t.Fatalf("wrong-password attempt %d: err = %v, want ErrInvalidCredentials", attempt, err)
		}
	}

	var reloaded models.User
	if err := db.Where("id = ?", user.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("cannot reload user: %v", err)
	}
	if reloaded.Status != models.UserStatusLocked {
		t.Errorf("status = %q after 3 failed attempts, want %q", reloaded.Status, models.UserStatusLocked)
	}
	if reloaded.LockedUntil == nil || !reloaded.IsLocked() {
		t.Error("LockedUntil was not set after 3 failed attempts")
	}
	if reloaded.LockedUntil != nil && time.Until(*reloaded.LockedUntil) > services.LockoutDuration {
		t.Errorf("lockout is longer than LockoutDuration (%v)", services.LockoutDuration)
	}

	// While locked, even the CORRECT password is refused.
	if _, err := auth.Login(user.Email, "correct-password-2"); !errors.Is(err, services.ErrUserLocked) {
		t.Fatalf("locked account login: err = %v, want ErrUserLocked", err)
	}
}

// TestAuthServiceLogin_SuccessAfterFailuresResetsCounter pins the
// coordination between a successful login and the failed-attempt counter: a
// user who mistypes and then signs in successfully starts over with a clean
// counter (neither the account nor the rate limiter accumulates stale
// failures).
func TestAuthServiceLogin_SuccessAfterFailuresResetsCounter(t *testing.T) {
	db := loginTestDB(t)
	user := seedLoginUser(t, db, "correct-password-3")

	auth := services.NewAuthService(repository.NewUserRepository(db))

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := auth.Login(user.Email, "still-wrong"); !errors.Is(err, services.ErrInvalidCredentials) {
			t.Fatalf("wrong-password attempt %d: err = %v, want ErrInvalidCredentials", attempt, err)
		}
	}

	if _, err := auth.Login(user.Email, "correct-password-3"); err != nil {
		t.Fatalf("successful login after typos failed: %v", err)
	}

	var reloaded models.User
	if err := db.Where("id = ?", user.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("cannot reload user: %v", err)
	}
	if reloaded.FailedLoginAttempts != 0 {
		t.Errorf("FailedLoginAttempts = %d after recovery login, want 0", reloaded.FailedLoginAttempts)
	}
	if reloaded.Status != models.UserStatusActive {
		t.Errorf("status = %q after recovery login, want %q", reloaded.Status, models.UserStatusActive)
	}

	// The account is fully usable again: two more wrong attempts must NOT
	// lock it (the counter truly restarted).
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := auth.Login(user.Email, "post-recovery-wrong"); !errors.Is(err, services.ErrInvalidCredentials) {
			t.Fatalf("post-recovery wrong attempt %d: err = %v", attempt, err)
		}
	}
	var afterRelock models.User
	if err := db.Where("id = ?", user.ID).First(&afterRelock).Error; err != nil {
		t.Fatalf("cannot reload user: %v", err)
	}
	if afterRelock.Status == models.UserStatusLocked {
		t.Error("account locked after only 2 post-recovery failures; counter was not reset")
	}
}

// TestAuthServiceLogin_UnknownIdentifierSafeResponse pins the safe
// invalid-credentials response for unknown identifiers (no user enumeration).
func TestAuthServiceLogin_UnknownIdentifierSafeResponse(t *testing.T) {
	db := loginTestDB(t)
	auth := services.NewAuthService(repository.NewUserRepository(db))

	if _, err := auth.Login("no-such-user-"+uuid.NewString(), "whatever"); !errors.Is(err, services.ErrInvalidCredentials) {
		t.Fatalf("unknown identifier: err = %v, want ErrInvalidCredentials", err)
	}
}
