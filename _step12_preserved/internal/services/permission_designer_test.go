package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

func actorWithRole(role string) *models.User {
	user := &models.User{Username: "actor"}
	user.Role = models.Role{Name: role}
	return user
}

// ---------------------------------------------------------------------------
// Permission designer role boundaries (spec sections 16, 20, 23)
// ---------------------------------------------------------------------------

func TestAdminCanUpdateStaffPermissions(t *testing.T) {
	svc := &PermissionService{} // pure guard — no repository access required
	if err := svc.CanManageRolePermissions(actorWithRole(models.RoleAdmin), models.RoleStaff); err != nil {
		t.Fatalf("Admin must be able to configure Staff permissions: %v", err)
	}
}

func TestAdminCanUpdateVolunteerPermissions(t *testing.T) {
	svc := &PermissionService{}
	if err := svc.CanManageRolePermissions(actorWithRole(models.RoleAdmin), models.RoleVolunteer); err != nil {
		t.Fatalf("Admin must be able to configure Volunteer permissions: %v", err)
	}
}

func TestAdminCanUpdatePartnerPermissions(t *testing.T) {
	svc := &PermissionService{}
	if err := svc.CanManageRolePermissions(actorWithRole(models.RoleAdmin), models.RolePartner); err != nil {
		t.Fatalf("Admin must be able to configure Partner operational permissions: %v", err)
	}
}

func TestAdminCannotModifyAdminPermissions(t *testing.T) {
	svc := &PermissionService{}
	if err := svc.CanManageRolePermissions(actorWithRole(models.RoleAdmin), models.RoleAdmin); err == nil {
		t.Fatal("Admin must NOT be able to modify the Admin role permission set")
	}
}

func TestAdminCannotModifySuperAdminPermissions(t *testing.T) {
	svc := &PermissionService{}
	if err := svc.CanManageRolePermissions(actorWithRole(models.RoleAdmin), models.RoleSuperAdmin); err == nil {
		t.Fatal("Admin must NOT be able to modify the Super Admin role permission set")
	}
}

func TestPartnerCannotModifyAnyRolePermissions(t *testing.T) {
	svc := &PermissionService{}

	// The pure role guard lets Partner manage operational roles it can
	// assign members to (Staff/Volunteer/Donor/Beneficiary), but Partner is
	// barred from the protected roles and — crucially — its default
	// permission set contains NO permission_management.* capability, so the
	// route gate rejects the designer entirely.
	for _, role := range []string{models.RoleSuperAdmin, models.RoleAdmin} {
		if err := svc.CanManageRolePermissions(actorWithRole(models.RolePartner), role); err == nil {
			t.Errorf("Partner must NOT be able to modify permissions of protected role %q", role)
		}
	}

	for _, name := range defaultRolePermissions()[models.RolePartner] {
		if strings.HasPrefix(name, "permission_management.") {
			t.Errorf("Partner default set must not contain %q (permission designer is Admin-only)", name)
		}
	}
}

func TestStaffAndVolunteerCannotModifyAnyRolePermissions(t *testing.T) {
	svc := &PermissionService{}

	for _, actor := range []string{models.RoleStaff, models.RoleVolunteer} {
		// Protected roles are always out of reach via the role guard.
		for _, role := range []string{models.RoleSuperAdmin, models.RoleAdmin} {
			if err := svc.CanManageRolePermissions(actorWithRole(actor), role); err == nil {
				t.Errorf("%s must NOT be able to modify permissions of protected role %q", actor, role)
			}
		}

		// The designer entry point is gated by permission_management.edit,
		// which neither role holds by default.
		for _, name := range defaultRolePermissions()[actor] {
			if strings.HasPrefix(name, "permission_management.") {
				t.Errorf("%s default set must not contain %q", actor, name)
			}
		}
	}
}

// TestAdminCannotGrantProtectedPermissions verifies both escalation barriers:
// the role-boundary guard rejects protected role targets and the permission
// filter blocks protected namespaces for non-Super-Admin roles.
func TestAdminCannotGrantProtectedPermissions(t *testing.T) {
	svc := &PermissionService{}

	// Designer guard: an Admin cannot touch the protected roles at all.
	if err := svc.CanManageRolePermissions(actorWithRole(models.RoleAdmin), models.RoleAdmin); err == nil {
		t.Fatal("Admin must not manage Admin role permissions")
	}

	protectedNames := []string{
		"admin.view",
		"admin.create",
		"admin.edit",
		"admin.activate",
		"admin.deactivate",
		"admin.delete.request",
		"admin.delete.approve",
		"admin.delete.reject",
		"super_admin.view",
		"super_admin.create",
		"system.settings.view",
		"system.settings.edit",
		"protected_rbac.manage",
	}

	allowed, blocked := FilterProtectedPermissions(protectedNames)
	if len(allowed) != 0 {
		t.Errorf("no protected permission may pass the filter, got allowed=%v", allowed)
	}
	if len(blocked) != len(protectedNames) {
		t.Errorf("all protected permissions must be blocked, got blocked=%v", blocked)
	}

	// Operational permissions pass untouched.
	allowed, blocked = FilterProtectedPermissions([]string{"student.view", "donation.approve"})
	if len(allowed) != 2 || len(blocked) != 0 {
		t.Errorf("operational permissions must pass the filter, got allowed=%v blocked=%v", allowed, blocked)
	}
}

// TestSetRolePermissionsByActorGuards exercises the actor-aware replacement
// entry point for the denial paths (no database required because every guard
// fires before repository access).
func TestSetRolePermissionsByActorGuards(t *testing.T) {
	svc := &PermissionService{}

	cases := []struct {
		name      string
		actorRole string
		target    string
		perms     []string
		wantErr   error
	}{
		{
			name:      "admin cannot modify admin role",
			actorRole: models.RoleAdmin,
			target:    models.RoleAdmin,
			perms:     []string{"users.view"},
			wantErr:   ErrPermissionDenied,
		},
		{
			name:      "admin cannot modify super admin role",
			actorRole: models.RoleAdmin,
			target:    models.RoleSuperAdmin,
			perms:     []string{"users.view"},
			wantErr:   ErrPermissionDenied,
		},
		{
			name:      "admin cannot inject protected permissions into staff",
			actorRole: models.RoleAdmin,
			target:    models.RoleStaff,
			perms:     []string{"student.view", "admin.create"},
			wantErr:   ErrProtectedPermission,
		},
		{
			name:      "partner cannot modify admin permissions",
			actorRole: models.RolePartner,
			target:    models.RoleAdmin,
			perms:     []string{"users.view"},
			wantErr:   ErrPermissionDenied,
		},
		{
			name:      "nil actor is denied",
			actorRole: "",
			target:    models.RoleStaff,
			perms:     []string{"student.view"},
			wantErr:   ErrPermissionDenied,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var actor *models.User
			if test.actorRole != "" {
				actor = actorWithRole(test.actorRole)
			}

			err := svc.SetRolePermissionsByActor(actor, test.target, test.perms)
			if err == nil {
				t.Fatalf("expected error %v, got nil", test.wantErr)
			}
			// Protected-permission violations are wrapped with the offending
			// names; compare with errors.Is for both plain and wrapped cases.
			if !errors.Is(err, test.wantErr) {
				t.Errorf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

// TestUserPermissionOverrideProtectedGrantsRejected pins the user-specific
// override guard predicate: overrides must never grant admin.* /
// super_admin.* / system.* / protected_rbac.* permissions (spec section 24).
func TestUserPermissionOverrideProtectedGrantsRejected(t *testing.T) {
	for _, name := range []string{
		"admin.create",
		"super_admin.create",
		"system.settings.edit",
		"protected_rbac.manage",
	} {
		if !IsProtectedPermission(name) {
			t.Errorf("%q must be treated as a protected permission", name)
		}
	}

	for _, name := range []string{"donation.approve", "student.view", "report.export"} {
		if IsProtectedPermission(name) {
			t.Errorf("%q must NOT be treated as a protected permission", name)
		}
	}
}

// TestAssignableRolesForExport verifies the exported view used by the UI for
// role choices. Admin may create Admin (multi-Admin), Partner (Manager),
// Staff, Volunteer, Donor, Beneficiary and Student — never Super Admin.
func TestAssignableRolesForExport(t *testing.T) {
	admin := AssignableRolesFor(models.RoleAdmin)
	want := []string{
		models.RoleAdmin,
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	}
	if len(admin) != len(want) {
		t.Fatalf("AssignableRolesFor(Admin) = %v, want %v", admin, want)
	}
	for i, role := range want {
		if admin[i] != role {
			t.Errorf("AssignableRolesFor(Admin)[%d] = %q, want %q", i, admin[i], role)
		}
	}

	if got := AssignableRolesFor(models.RoleDonor); len(got) != 0 {
		t.Errorf("AssignableRolesFor(Donor) = %v, want empty", got)
	}
}
