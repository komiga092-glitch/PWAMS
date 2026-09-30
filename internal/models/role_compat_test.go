package models_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// Phase 2 regression tests for the Partner → Manager compatibility layer
// (§13). Partner is legacy/internal only; the user-facing name is Manager.
func TestNormalizeRoleInput(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Partner", models.RoleManager},
		{"partner", models.RoleManager},
		{" PARTNER ", models.RoleManager},
		{" Manager ", models.RoleManager},
		{"Manager", models.RoleManager},
		{models.RoleAdmin, models.RoleAdmin},
		{"  " + models.RoleStaff + "  ", models.RoleStaff},
		{"", ""},
	}

	for _, tc := range cases {
		if got := models.NormalizeRoleInput(tc.input); got != tc.want {
			t.Errorf("NormalizeRoleInput(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsManagerRole(t *testing.T) {
	if !models.IsManagerRole("Partner") {
		t.Error(`IsManagerRole("Partner") must be true (legacy alias)`)
	}
	if !models.IsManagerRole(models.RoleManager) {
		t.Error("IsManagerRole(Manager) must be true")
	}
	if models.IsManagerRole(models.RoleStaff) {
		t.Error("IsManagerRole(Staff) must be false")
	}
}

func TestRoleDisplayName(t *testing.T) {
	if got := models.RoleDisplayName("Partner"); got != models.RoleManager {
		t.Errorf("RoleDisplayName(Partner) = %q, want Manager", got)
	}
	if got := models.RoleDisplayName(models.RoleManager); got != models.RoleManager {
		t.Errorf("RoleDisplayName(Manager) = %q, want Manager", got)
	}
	if got := models.RoleDisplayName(models.RoleDonor); got != models.RoleDonor {
		t.Errorf("RoleDisplayName(Donor) = %q, want Donor", got)
	}
}