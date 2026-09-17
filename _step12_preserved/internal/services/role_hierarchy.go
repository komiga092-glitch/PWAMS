package services

import (
	"strings"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// This file is the single source of truth for the PWAMS role hierarchy.
//
//	 Super Admin  (system owner — manages Admin accounts)
//	     |
//	  Admin       (platform administrator — manages every normal NGO account;
//	               never Super Admin)
//	     |
//	 Partner     (user-facing "Manager" — operational management)
//	     |
//	 Staff / Volunteer   (field operations)
//	     |
//	 Donor / Beneficiary / Student (operational users / self-service)
//
// Final creation rules enforced by CanAssignRole:
//
//	SUPER_ADMIN -> ADMIN, PARTNER (Manager), STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	ADMIN       -> ADMIN, PARTNER (Manager), STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	PARTNER     -> STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	STAFF       -> DONOR, BENEFICIARY, STUDENT
//	VOLUNTEER   -> DONOR, BENEFICIARY, STUDENT
//	DONOR / BENEFICIARY / STUDENT -> nothing (self-service only)
//
// A role can never create an account above its own level, which makes
// privilege escalation through account creation impossible: in particular a
// Super Admin cannot mint additional Super Admins, and an Admin can never
// create (or be assigned) a Super Admin account. Admin accounts are a
// peer-managed level — both the Super Admin and other Admins may create them
// (multi-Admin support); the boundary that is never crossed is the Super
// Admin role.
//
// CanManageAccountRole extends the same boundary to account-management
// actions (view / edit / activate / deactivate / delete / password reset):
// an actor may only manage accounts whose role it could assign, plus
// operational roles it is responsible for. Admin accounts can be managed by
// a Super Admin or an Admin; Super Admin accounts can only ever be managed
// by a Super Admin.

// assignableRolesFor maps every actor role to the target roles it may
// create/assign accounts for.
var assignableRolesFor = map[string][]string{
	models.RoleSuperAdmin: {
		models.RoleAdmin,
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	},
	models.RoleAdmin: {
		models.RoleAdmin,
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	},
	models.RolePartner: {
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	},
	models.RoleStaff:     {models.RoleDonor, models.RoleBeneficiary, models.RoleStudent},
	models.RoleVolunteer: {models.RoleDonor, models.RoleBeneficiary, models.RoleStudent},
	// Donor, Beneficiary, Student and unknown roles may create nothing.
}

// AssignableRolesFor returns the target roles an actor with actorRole may
// create accounts for. It is the exported view over assignableRolesFor and
// is used by handlers to render role choices that are always consistent
// with the backend hierarchy (the backend remains the enforcement point).
func AssignableRolesFor(actorRole string) []string {
	targets, ok := assignableRolesFor[actorRole]
	if !ok {
		return []string{}
	}
	return append([]string{}, targets...)
}

// CanAssignRole returns whether an actor with actorRole may create an
// account with targetRole, enforcing the strict hierarchy documented above.
func CanAssignRole(actorRole, targetRole string) bool {
	targets, ok := assignableRolesFor[actorRole]
	if !ok {
		return false
	}

	for _, target := range targets {
		if target == targetRole {
			return true
		}
	}

	return false
}

// managedRolesFor maps every actor role to the target roles whose accounts
// it may administer (view/edit/activate/deactivate/delete/reset password).
var managedRolesFor = map[string][]string{
	models.RoleSuperAdmin: models.AllRoles(),
	models.RoleAdmin: {
		models.RoleAdmin,
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	},
	models.RolePartner: {
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	},
	models.RoleStaff:     {models.RoleDonor, models.RoleBeneficiary, models.RoleStudent},
	models.RoleVolunteer: {models.RoleDonor, models.RoleBeneficiary, models.RoleStudent},
	// Donor, Beneficiary, Student and unknown roles may manage nothing.
}

// CanManageAccountRole returns whether an actor with actorRole may perform
// account-management actions on an account of targetRole. Admin accounts can
// be managed by a Super Admin or an Admin (multi-Admin support); Super Admin
// accounts only by a Super Admin; Partner accounts only by an Admin or
// Super Admin.
func CanManageAccountRole(actorRole, targetRole string) bool {
	targets, ok := managedRolesFor[actorRole]
	if !ok {
		return false
	}

	for _, target := range targets {
		if target == targetRole {
			return true
		}
	}

	return false
}

// Protected permission namespaces. Permissions in these namespaces define
// who may administer the system itself (Admin accounts, Super Admin
// accounts, system configuration and the RBAC layer). super_admin.*,
// system.* and protected_rbac.* may ONLY ever be granted to the Super Admin
// role. admin.* is the account-management namespace for the Admin role:
// since the Admin role manages other Admin accounts (multi-Admin support) it
// holds admin.*, so admin.* may be granted to the Super Admin and Admin
// roles only. No protected permission can be granted to the Partner / Staff
// / Volunteer / Donor / Beneficiary / Student roles, or to users through
// overrides — not even by an Admin with permission_management.edit, which
// closes the privilege-escalation path through the permission-management
// API.
const (
	protectedNamespaceAdmin         = "admin."
	protectedNamespaceSuperAdmin    = "super_admin."
	protectedNamespaceSystem        = "system."
	protectedNamespaceProtectedRBAC = "protected_rbac."
)

// RoleMayHoldPermission reports whether the given role may be granted the
// permission. Super Admin may hold every permission; the Admin role may hold
// the admin.* namespace (it manages Admin accounts) but no other protected
// namespace; every other role may hold no protected permission at all.
func RoleMayHoldPermission(roleName, permissionName string) bool {
	if roleName == models.RoleSuperAdmin {
		return true
	}

	if !IsProtectedPermission(permissionName) {
		return true
	}

	return roleName == models.RoleAdmin &&
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(permissionName)), protectedNamespaceAdmin)
}

// filterPermissionsForRole splits a permission name list into the names the
// target role may hold and the protected names that must be rejected for it.
// It is the role-aware counterpart of FilterProtectedPermissions and is used
// wherever a permission set is applied to a concrete role.
func filterPermissionsForRole(roleName string, names []string) (allowed, blocked []string) {
	allowed = make([]string, 0, len(names))
	blocked = make([]string, 0)

	for _, name := range names {
		if RoleMayHoldPermission(roleName, name) {
			allowed = append(allowed, name)
			continue
		}
		blocked = append(blocked, name)
	}

	return allowed, blocked
}

// IsProtectedPermission returns whether the permission belongs to one of the
// protected namespaces that define who may administer the system itself.
//
// The admin.* namespace deserves special attention: it is the Admin
// *account-management* namespace (Admin-to-Admin account management) and is
// held by both the Super Admin and Admin roles (multi-Admin support — see
// RoleMayHoldPermission). It is still classified as protected so that it can
// never be granted beyond those two roles through the permission designer,
// role permission sets, or user-specific overrides: handing admin.* to an
// operational role would let that role manage Admin accounts, which is a
// privilege-escalation path. This is deliberately different from (and
// independent of) modifying the Admin *role's permission definition* itself,
// which is reserved for the Super Admin role via
// PermissionService.CanManageRolePermissions.
//
// super_admin.*, system.* and protected_rbac.* remain Super-Admin-exclusive.
//
// Use RoleMayHoldPermission for role-aware decisions; this function reports
// the broad "is this permission system-sensitive" classification used by
// catalog filtering and user-override rejection.
func IsProtectedPermission(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))

	for _, prefix := range []string{
		protectedNamespaceAdmin,
		protectedNamespaceSuperAdmin,
		protectedNamespaceSystem,
		protectedNamespaceProtectedRBAC,
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

// FilterProtectedPermissions splits a permission name list into the names
// that may be assigned to a non-SuperAdmin, non-Admin role and the protected
// names that must be rejected. It is the universal (non-role-aware) variant:
// for a concrete target role use filterPermissionsForRole, which also allows
// the Admin role to hold admin.* (multi-Admin support).
func FilterProtectedPermissions(names []string) (allowed, blocked []string) {
	allowed = make([]string, 0, len(names))
	blocked = make([]string, 0)

	for _, name := range names {
		if IsProtectedPermission(name) {
			blocked = append(blocked, name)
			continue
		}
		allowed = append(allowed, name)
	}

	return allowed, blocked
}
