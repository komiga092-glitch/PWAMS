package handlers

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// TestCanCreateRole verifies the backend account-creation hierarchy
// (delegated to services.CanAssignRole — see services/role_hierarchy.go):
//
//	SUPER_ADMIN -> ADMIN, MANAGER (Partner role), STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	ADMIN       -> ADMIN, MANAGER (Partner role), STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	MANAGER (Partner role) -> STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	STAFF       -> DONOR, BENEFICIARY, STUDENT
//	VOLUNTEER   -> DONOR, BENEFICIARY, STUDENT
//	anything else -> nothing (unauthorised)
func TestCanCreateRole(t *testing.T) {
	allowed := map[string]map[string]bool{
		models.RoleSuperAdmin: {
			models.RoleAdmin:       true,
			models.RolePartner:     true,
			models.RoleStaff:       true,
			models.RoleVolunteer:   true,
			models.RoleDonor:       true,
			models.RoleBeneficiary: true,
			models.RoleStudent:     true,
		},
		models.RoleAdmin: {
			models.RoleAdmin:       true,
			models.RolePartner:     true,
			models.RoleStaff:       true,
			models.RoleVolunteer:   true,
			models.RoleDonor:       true,
			models.RoleBeneficiary: true,
			models.RoleStudent:     true,
		},
		models.RolePartner: {
			models.RoleStaff:       true,
			models.RoleVolunteer:   true,
			models.RoleDonor:       true,
			models.RoleBeneficiary: true,
			models.RoleStudent:     true,
		},
		models.RoleStaff: {
			models.RoleDonor:       true,
			models.RoleBeneficiary: true,
			models.RoleStudent:     true,
		},
		models.RoleVolunteer: {
			models.RoleDonor:       true,
			models.RoleBeneficiary: true,
			models.RoleStudent:     true,
		},
		models.RoleDonor:       {},
		models.RoleBeneficiary: {},
		models.RoleStudent:     {},
	}

	for _, actor := range models.AllRoles() {
		for _, target := range models.AllRoles() {
			want := allowed[actor][target]
			if got := canCreateRole(actor, target); got != want {
				t.Errorf("canCreateRole(%q, %q) = %v, want %v", actor, target, got, want)
			}
		}
	}

	// Unknown roles must never be allowed to create anything.
	if canCreateRole("Ghost", models.RoleAdmin) {
		t.Error("unknown actor role must not be able to create accounts")
	}
}

// TestPrivilegeEscalationViaAccountCreationRegression pins the hierarchy
// guards that make privilege escalation through account creation impossible.
// The one sanctioned same-level creation is Admin -> Admin (multi-Admin),
// asserted by TestAdminCanCreateAdmin.
func TestPrivilegeEscalationViaAccountCreationRegression(t *testing.T) {
	if canCreateRole(models.RoleSuperAdmin, models.RoleSuperAdmin) {
		t.Error("Super Admin must not be able to create another Super Admin")
	}
	if canCreateRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Error("Admin must not be able to create a Super Admin")
	}
	if canCreateRole(models.RolePartner, models.RolePartner) {
		t.Error("Partner must not be able to create another Partner")
	}
}

// TestStaffAndVolunteerCannotCreateAdminAccounts ensures operational roles
// can only ever register donor/beneficiary/student accounts — never
// administrative ones (Super Admin / Admin / Manager / each other).
func TestStaffAndVolunteerCannotCreateAdminAccounts(t *testing.T) {
	adminish := []string{
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
	}

	for _, actor := range []string{models.RoleStaff, models.RoleVolunteer} {
		for _, target := range adminish {
			if canCreateRole(actor, target) {
				t.Errorf("%q must not be able to create a %q account", actor, target)
			}
		}
	}
}

// TestRequiredTargetPermission verifies the role-specific permission mapping
// used by the account-management endpoints: Admin targets require admin.*,
// Partner targets require partner.*, everything else requires no additional
// permission beyond the generic users.* route permission.
//
// Per the final role model, CREATING an Admin or Partner (Manager) account
// requires NO separate permission — creation is governed by the role hierarchy
// alone (services.CanAssignRole). The "create" action therefore maps to ""
// (no additional permission) for both Admin and Partner targets, while
// editing, activating, deactivating and deleting keep their granular gates.
func TestRequiredTargetPermission(t *testing.T) {
	adminActions := map[string]string{
		"view":       "admin.view",
		"create":     "", // hierarchy-gated, no separate permission
		"edit":       "admin.edit",
		"activate":   "admin.activate",
		"deactivate": "admin.deactivate",
		// Direct deletion of an Admin account requires the same permission
		// as approving a workflow deletion request (execution authority).
		"delete":         "admin.delete.approve",
		"delete.request": "admin.delete.request",
		"delete.approve": "admin.delete.approve",
		"delete.reject":  "admin.delete.reject",
	}
	for action, want := range adminActions {
		if got := requiredTargetPermission(models.RoleAdmin, action); got != want {
			t.Errorf("requiredTargetPermission(Admin, %q) = %q, want %q", action, got, want)
		}
	}

	partnerActions := map[string]string{
		"view":       "partner.view",
		"create":     "", // hierarchy-gated, no separate permission
		"edit":       "partner.edit",
		"activate":   "partner.activate",
		"deactivate": "partner.deactivate",
		"delete":     "partner.delete",
	}
	for action, want := range partnerActions {
		if got := requiredTargetPermission(models.RolePartner, action); got != want {
			t.Errorf("requiredTargetPermission(Partner, %q) = %q, want %q", action, got, want)
		}
	}

	for _, role := range []string{
		models.RoleSuperAdmin,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
		"Ghost",
	} {
		if got := requiredTargetPermission(role, "delete"); got != "" {
			t.Errorf("requiredTargetPermission(%q, %q) = %q, want \"\"", role, "delete", got)
		}
	}
}

// TestAccountManagementAction verifies the role-specific audit action names
// used for Admin / Partner account lifecycle traceability.
func TestAccountManagementAction(t *testing.T) {
	if got := accountManagementAction("CREATE", models.RoleAdmin); got != "CREATE_ADMIN" {
		t.Errorf("accountManagementAction(CREATE, Admin) = %q, want %q", got, "CREATE_ADMIN")
	}
	if got := accountManagementAction("DELETE", models.RolePartner); got != "DELETE_PARTNER" {
		t.Errorf("accountManagementAction(DELETE, Partner) = %q, want %q", got, "DELETE_PARTNER")
	}
	if got := accountManagementAction("ACTIVATE", models.RoleStaff); got != "ACTIVATE" {
		t.Errorf("accountManagementAction(ACTIVATE, Staff) = %q, want %q", got, "ACTIVATE")
	}
}

// TestStatusAuditAction verifies the status change audit action mapping.
func TestStatusAuditAction(t *testing.T) {
	cases := map[string]string{
		models.UserStatusActive:   "ACTIVATE",
		models.UserStatusDisabled: "DEACTIVATE",
		models.UserStatusLocked:   "STATUS_CHANGE",
	}
	for status, want := range cases {
		if got := statusAuditAction(status); got != want {
			t.Errorf("statusAuditAction(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestManagedAuditAction verifies the account-management audit action
// convention produced for the role-scoped management endpoints
// (CREATE_ADMIN, UPDATE_ADMIN, ACTIVATE_ADMIN, ... — Part 8).
func TestManagedAuditAction(t *testing.T) {
	cases := []struct {
		verb string
		role string
		want string
	}{
		{"CREATED", models.RoleAdmin, "CREATE_ADMIN"},
		{"UPDATED", models.RoleAdmin, "UPDATE_ADMIN"},
		{"ACTIVATE", models.RoleAdmin, "ACTIVATE_ADMIN"},
		{"DEACTIVATE", models.RoleAdmin, "DEACTIVATE_ADMIN"},
		{"DELETED", models.RoleAdmin, "DELETE_ADMIN"},
		{"CREATED", models.RolePartner, "CREATE_PARTNER"},
		{"ACTIVATED", models.RolePartner, "ACTIVATE_PARTNER"},
		{"DEACTIVATED", models.RolePartner, "DEACTIVATE_PARTNER"},
		{"CREATED", models.RoleStaff, "CREATE_STAFF"},
		{"UPDATED", models.RoleStaff, "UPDATE_STAFF"},
		{"CREATED", models.RoleVolunteer, "CREATE_VOLUNTEER"},
		{"UPDATED", models.RoleVolunteer, "UPDATE_VOLUNTEER"},
	}

	for _, test := range cases {
		if got := managedAuditAction(test.verb, test.role); got != test.want {
			t.Errorf("managedAuditAction(%q, %q) = %q, want %q", test.verb, test.role, got, test.want)
		}
	}
}
