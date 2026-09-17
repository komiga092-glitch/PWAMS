package services

import (
	"sort"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// TestSuperAdminHasAllPermissions proves the central security invariant of
// the final role model: the Super Admin (system owner) is seeded with the
// COMPLETE permission catalog — missing permissions = 0.
//
// The default permission model derives the Super Admin set from
// allPermissions(), so every permission added to the catalog in the future
// is picked up automatically by SeedDefaults (which additionally repairs an
// already-configured Super Admin role to the full catalog on every startup).
func TestSuperAdminHasAllPermissions(t *testing.T) {
	catalog := permissionNames(allPermissions())
	superAdmin := defaultRolePermissions()[models.RoleSuperAdmin]

	catalogSet := make(map[string]bool, len(catalog))
	for _, name := range catalog {
		catalogSet[name] = true
	}

	superAdminSet := make(map[string]bool, len(superAdmin))
	for _, name := range superAdmin {
		superAdminSet[name] = true
	}

	var missing []string
	for _, name := range catalog {
		if !superAdminSet[name] {
			missing = append(missing, name)
		}
	}

	var extra []string
	for _, name := range superAdmin {
		if !catalogSet[name] {
			extra = append(extra, name)
		}
	}

	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf("Super Admin missing permissions = %d, want 0: %v", len(missing), missing)
	}

	if len(extra) != 0 {
		sort.Strings(extra)
		t.Fatalf("Super Admin default must not contain names outside the catalog: %v", extra)
	}
}

// The Super Admin role may hold every permission in the catalog, including
// every protected namespace (admin.*, super_admin.*, system.*,
// protected_rbac.*) — no permission can ever be out of the system owner's
// reach.
func TestSuperAdminMayHoldEveryPermission(t *testing.T) {
	for _, p := range allPermissions() {
		if !RoleMayHoldPermission(models.RoleSuperAdmin, p.Name) {
			t.Errorf("Super Admin must be allowed to hold permission %q", p.Name)
		}
		if !IsProtectedPermission(p.Name) {
			continue
		}
		if !RoleMayHoldPermission(models.RoleSuperAdmin, p.Name) {
			t.Errorf("protected permission %q must be holdable by the Super Admin role", p.Name)
		}
	}
}

// The Admin role must hold the full admin.* account-management namespace
// (final rule: Admin manages other Admin accounts) while never holding the
// Super-Admin-exclusive namespaces. Everything else in the catalog that is
// not protected must be available to the Admin role's permission model.
func TestAdminDefaultPermissionBoundary(t *testing.T) {
	admin := defaultRolePermissions()[models.RoleAdmin]
	held := make(map[string]bool, len(admin))
	for _, name := range admin {
		held[name] = true
	}

	adminNamespace := []string{
		"admin.view", "admin.create", "admin.edit",
		"admin.activate", "admin.deactivate",
		"admin.delete.request", "admin.delete.approve", "admin.delete.reject",
	}
	for _, name := range adminNamespace {
		if !held[name] {
			t.Errorf("Admin default must hold %q", name)
		}
	}

	for _, p := range allPermissions() {
		mayHold := RoleMayHoldPermission(models.RoleAdmin, p.Name)
		if !mayHold && held[p.Name] {
			t.Errorf("Admin default holds %q but RoleMayHoldPermission forbids it", p.Name)
		}
		if mayHold && IsProtectedPermission(p.Name) {
			if !held[p.Name] {
				t.Errorf("Admin may hold %q (protected admin.* namespace) and the default set must include it", p.Name)
			}
		}
	}

	superAdminExclusive := []string{
		"super_admin.view", "super_admin.create", "super_admin.edit",
		"super_admin.deactivate",
		"system.settings.view", "system.settings.edit", "system.maintenance",
		"protected_rbac.manage",
	}
	for _, name := range superAdminExclusive {
		if held[name] {
			t.Errorf("Admin default must NOT hold the Super-Admin-exclusive permission %q", name)
		}
	}
}
