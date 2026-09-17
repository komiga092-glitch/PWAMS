package services_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

// seedLockableUser inserts an account with a real bcrypt password hash in the
// requested status and registers cleanup so repeated runs never collide on
// the unique email/username indexes.
func seedLockableUser(t *testing.T, db *gorm.DB, password, status string) models.User {
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
		Username:     "locktest_" + suffix,
		Email:        "locktest_" + suffix + "@pwams.local",
		FullName:     "Lockout Test Account",
		PasswordHash: hash,
		RoleID:       role.ID,
		Status:       status,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create lockout test user: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Where("user_id = ?", user.ID).Delete(&models.AuditLog{}).Error
		_ = db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{}).Error
	})

	return user
}

// tripLockout drives the account into the exact state production leaves it in
// after the configured number of failed attempts: Status=Locked with a fresh
// lockout window.
func tripLockout(t *testing.T, auth *services.AuthService, email string) {
	t.Helper()

	for attempt := 1; attempt <= services.MaxFailedLoginAttempts; attempt++ {
		if _, err := auth.Login(email, "wrong-password"); !errors.Is(err, services.ErrInvalidCredentials) {
			t.Fatalf("wrong-password attempt %d: err = %v, want ErrInvalidCredentials", attempt, err)
		}
	}
}

// expireLockout moves locked_until into the past while leaving Status=Locked,
// which is precisely the state an account is in once its lockout window has
// passed — no code path resets the status on expiry.
func expireLockout(t *testing.T, db *gorm.DB, userID uuid.UUID) {
	t.Helper()

	expired := time.Now().Add(-time.Minute)
	if err := db.Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{"locked_until": expired}).Error; err != nil {
		t.Fatalf("cannot expire lockout: %v", err)
	}
}

// TestAuthServiceLogin_ExpiredLockAllowsRecoveryLogin pins the expired-lockout
// recovery rule: an account whose lockout window has passed (IsLocked() ==
// false) but whose persisted status is still Locked MUST be able to sign in
// with the correct password, and the successful login must restore the
// account to Active with a cleared counter and lock. Before this was fixed
// the account was permanently locked out ("user account status is invalid"),
// which is how the non-admin role accounts became unable to log in.
func TestAuthServiceLogin_ExpiredLockAllowsRecoveryLogin(t *testing.T) {
	db := loginTestDB(t)
	user := seedLockableUser(t, db, "correct-password-1", models.UserStatusActive)

	auth := services.NewAuthService(repository.NewUserRepository(db))

	tripLockout(t, auth, user.Email)
	expireLockout(t, db, user.ID)

	loggedIn, err := auth.Login(user.Email, "correct-password-1")
	if err != nil {
		t.Fatalf("recovery login after lock expiry failed: %v", err)
	}
	if loggedIn.ID != user.ID {
		t.Fatalf("recovery login returned user %s, want %s", loggedIn.ID, user.ID)
	}

	var reloaded models.User
	if err := db.Where("id = ?", user.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("cannot reload user: %v", err)
	}
	if reloaded.Status != models.UserStatusActive {
		t.Errorf("status = %q after recovery login, want %q", reloaded.Status, models.UserStatusActive)
	}
	if reloaded.FailedLoginAttempts != 0 {
		t.Errorf("FailedLoginAttempts = %d after recovery login, want 0", reloaded.FailedLoginAttempts)
	}
	if reloaded.LockedUntil != nil {
		t.Errorf("LockedUntil = %v after recovery login, want nil", reloaded.LockedUntil)
	}
}

// TestAuthServiceLogin_ExpiredLockStillRejectsWrongPassword pins the security
// side of the expired-lock rule: recovery is only possible with the CORRECT
// password. A wrong password on an expired-locked account re-locks it for a
// fresh lockout window, so the fix does not weaken lockout security.
func TestAuthServiceLogin_ExpiredLockStillRejectsWrongPassword(t *testing.T) {
	db := loginTestDB(t)
	user := seedLockableUser(t, db, "correct-password-2", models.UserStatusActive)

	auth := services.NewAuthService(repository.NewUserRepository(db))

	tripLockout(t, auth, user.Email)
	expireLockout(t, db, user.ID)

	if _, err := auth.Login(user.Email, "still-wrong"); !errors.Is(err, services.ErrInvalidCredentials) {
		t.Fatalf("wrong password on expired-locked account: err = %v, want ErrInvalidCredentials", err)
	}

	var relocked models.User
	if err := db.Where("id = ?", user.ID).First(&relocked).Error; err != nil {
		t.Fatalf("cannot reload user: %v", err)
	}
	if relocked.Status != models.UserStatusLocked {
		t.Errorf("status = %q after wrong password on expired-locked account, want %q (re-locked)",
			relocked.Status, models.UserStatusLocked)
	}
	if relocked.LockedUntil == nil || !relocked.IsLocked() {
		t.Error("account was not re-locked after wrong password on expired-locked account")
	}

	// The correct password still recovers the account once the fresh lock
	// expires.
	expireLockout(t, db, user.ID)
	if _, err := auth.Login(user.Email, "correct-password-2"); err != nil {
		t.Fatalf("recovery login after second lock expiry failed: %v", err)
	}
}

// TestAuthServiceLogin_ActiveLockStillBlocksCorrectPassword pins that an
// unexpired lockout still refuses even the correct password, so the
// expired-lock recovery fix leaves the lockout itself untouched.
func TestAuthServiceLogin_ActiveLockStillBlocksCorrectPassword(t *testing.T) {
	db := loginTestDB(t)
	user := seedLockableUser(t, db, "correct-password-3", models.UserStatusActive)

	auth := services.NewAuthService(repository.NewUserRepository(db))

	tripLockout(t, auth, user.Email)

	if _, err := auth.Login(user.Email, "correct-password-3"); !errors.Is(err, services.ErrUserLocked) {
		t.Fatalf("active-locked login: err = %v, want ErrUserLocked", err)
	}
}
