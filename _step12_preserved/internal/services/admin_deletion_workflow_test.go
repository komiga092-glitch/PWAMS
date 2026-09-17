package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// ---------------------------------------------------------------------------
// Admin account deletion workflow guards (privilege-escalation tests)
// ---------------------------------------------------------------------------

// TestCanRequestAdminDeletion_Matrix pins the workflow target boundaries:
// only Admin accounts may be targeted, and only by actors that may manage
// Admin accounts (Super Admin or Admin — multi-Admin). Every other
// combination is rejected, which makes privilege escalation through the
// deletion workflow impossible.
func TestCanRequestAdminDeletion_Matrix(t *testing.T) {
	allowed := map[string]map[string]bool{
		models.RoleSuperAdmin: {models.RoleAdmin: true},
		models.RoleAdmin:      {models.RoleAdmin: true},
	}

	for _, actor := range models.AllRoles() {
		for _, target := range models.AllRoles() {
			err := canRequestAdminDeletion(actor, target)
			want := allowed[actor][target]

			if want && err != nil {
				t.Errorf("canRequestAdminDeletion(%q, %q) = %v, want nil", actor, target, err)
			}
			if !want && err == nil {
				t.Errorf("canRequestAdminDeletion(%q, %q) = nil, want error", actor, target)
			}
		}
	}

	// The specific escalation paths that must never open.
	if err := canRequestAdminDeletion(models.RoleAdmin, models.RoleSuperAdmin); !errors.Is(err, ErrDeletionTargetRole) {
		t.Errorf("Admin requesting deletion of a Super Admin must fail with ErrDeletionTargetRole, got %v", err)
	}
	if err := canRequestAdminDeletion(models.RolePartner, models.RoleAdmin); !errors.Is(err, ErrDeletionHierarchy) {
		t.Errorf("Partner requesting deletion of an Admin must fail with ErrDeletionHierarchy, got %v", err)
	}
	if err := canRequestAdminDeletion(models.RoleSuperAdmin, models.RoleSuperAdmin); !errors.Is(err, ErrDeletionTargetRole) {
		t.Errorf("Super Admin requesting deletion of a Super Admin must fail with ErrDeletionTargetRole, got %v", err)
	}
	if err := canRequestAdminDeletion("Ghost", models.RoleAdmin); !errors.Is(err, ErrDeletionHierarchy) {
		t.Errorf("unknown roles must never pass the workflow guard, got %v", err)
	}
}

// TestCanResolveAdminDeletion_FourEyes pins the separation-of-duties rules:
// a deletion request can never be approved or rejected by the account that
// raised it (four-eyes), nor by the account the request targets — the subject
// of a deletion decision never decides their own removal.
func TestCanResolveAdminDeletion_FourEyes(t *testing.T) {
	requester := uuid.New()
	target := uuid.New()

	if err := canResolveAdminDeletion(requester, requester, target); !errors.Is(err, ErrDeletionSelfApproval) {
		t.Errorf("self-resolution must fail with ErrDeletionSelfApproval, got %v", err)
	}
	if err := canResolveAdminDeletion(requester, target, target); !errors.Is(err, ErrDeletionTargetApproval) {
		t.Errorf("the target resolving their own deletion request must fail with ErrDeletionTargetApproval, got %v", err)
	}
	if err := canResolveAdminDeletion(requester, uuid.New(), target); err != nil {
		t.Errorf("resolution by a different actor must be allowed, got %v", err)
	}
}

// TestAdminDeletionPermissionsProtectedFromLowerRoles verifies that the new
// deletion-workflow permissions remain protected: the Admin role may hold
// them (multi-Admin), but they can never be granted to any operational role
// through role permission sets or user overrides.
func TestAdminDeletionPermissionsProtectedFromLowerRoles(t *testing.T) {
	trio := []string{
		"admin.delete.request",
		"admin.delete.approve",
		"admin.delete.reject",
	}

	for _, name := range trio {
		if !IsProtectedPermission(name) {
			t.Errorf("%q must remain a protected permission", name)
		}
		if !RoleMayHoldPermission(models.RoleSuperAdmin, name) {
			t.Errorf("Super Admin must be able to hold %q", name)
		}
		if !RoleMayHoldPermission(models.RoleAdmin, name) {
			t.Errorf("Admin must be able to hold %q (multi-Admin)", name)
		}
	}

	for _, role := range []string{
		models.RolePartner, models.RoleStaff, models.RoleVolunteer,
		models.RoleDonor, models.RoleBeneficiary, models.RoleStudent,
	} {
		allowed, blocked := filterPermissionsForRole(role, trio)
		if len(allowed) != 0 || len(blocked) != len(trio) {
			t.Errorf("%s: deletion-workflow permissions must all be blocked, got allowed=%v blocked=%v", role, allowed, blocked)
		}
		for _, name := range trio {
			if RoleMayHoldPermission(role, name) {
				t.Errorf("%s must NOT be able to hold %q", role, name)
			}
		}
	}
}

// TestAdminRoleDefinitionIsSuperAdminOnly pins the separation between
// A) managing Admin user accounts (admin.*, held by Admin) and B) modifying
// the Admin role's security/permission definition (reserved for Super Admin).
// Holding the full admin.* namespace — including the deletion workflow —
// must not confer any designer capability over the Admin role itself.
func TestAdminRoleDefinitionIsSuperAdminOnly(t *testing.T) {
	svc := &PermissionService{}
	admin := actorWithRole(models.RoleAdmin)

	// (A) the Admin may manage Admin accounts — the whole admin.* namespace
	// is in its default set.
	defaults := defaultRolePermissions()
	held := make(map[string]bool, len(defaults[models.RoleAdmin]))
	for _, name := range defaults[models.RoleAdmin] {
		held[name] = true
	}
	for _, name := range []string{
		"admin.view", "admin.create", "admin.edit", "admin.activate",
		"admin.deactivate", "admin.delete.request", "admin.delete.approve",
		"admin.delete.reject",
	} {
		if !held[name] {
			t.Errorf("Admin default set must include %q (capability A)", name)
		}
	}

	// (B) yet the Admin can never redesign the Admin role's permission set.
	if err := svc.CanManageRolePermissions(admin, models.RoleAdmin); err == nil {
		t.Error("Admin must not modify the Admin role permission definition (capability B)")
	}
	if err := svc.CanManageRolePermissions(admin, models.RoleSuperAdmin); err == nil {
		t.Error("Admin must not modify the Super Admin role permission definition")
	}

	// Injecting the deletion workflow permissions (or any admin.* capability)
	// into an operational role through the designer is rejected outright.
	for _, name := range []string{
		"admin.delete.request", "admin.delete.approve", "admin.delete.reject", "admin.create",
	} {
		err := svc.SetRolePermissionsByActor(admin, models.RolePartner, []string{"student.view", name})
		if !errors.Is(err, ErrProtectedPermission) {
			t.Errorf("injecting %q into the Partner role must fail with ErrProtectedPermission, got %v", name, err)
		}
	}
}

// TestAdminDefaultsExcludeLegacyDeleteAndSystemNamespaces verifies that the
// escalation-prone namespaces never leak into the Admin default set and that
// the removed monolithic admin.delete permission is not re-introduced.
func TestAdminDefaultsExcludeLegacyDeleteAndSystemNamespaces(t *testing.T) {
	for _, name := range defaultRolePermissions()[models.RoleAdmin] {
		switch {
		case name == "admin.delete":
			t.Errorf("legacy permission %q must not be re-introduced into the Admin default set", name)
		case strings.HasPrefix(name, "super_admin."),
			strings.HasPrefix(name, "system."),
			strings.HasPrefix(name, "protected_rbac."):
			t.Errorf("Admin default set must not contain %q", name)
		}
	}
}
