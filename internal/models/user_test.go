package models_test

import (
	"testing"
	"time"

	"github.com/komiga092-glitch/pwams/internal/models"
)

func TestUserIsLocked_NilTime(t *testing.T) {
	user := &models.User{
		LockedUntil: nil,
	}
	if user.IsLocked() {
		t.Error("User with nil LockedUntil should not be locked")
	}
}

func TestUserIsLocked_PastTime(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	user := &models.User{
		LockedUntil: &past,
	}
	if user.IsLocked() {
		t.Error("User with past LockedUntil should not be locked")
	}
}

func TestUserIsLocked_FutureTime(t *testing.T) {
	future := time.Now().Add(30 * time.Minute)
	user := &models.User{
		LockedUntil: &future,
	}
	if !user.IsLocked() {
		t.Error("User with future LockedUntil should be locked")
	}
}
