package services

import (
	"reflect"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// TestCanAssignRole verifies the strict account-creation hierarchy:
//
//	SUPER_ADMIN -> ADMIN, PARTNER (Manager), STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	ADMIN       -> ADMIN, PARTNER (Manager), STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	PARTNER     -> STAFF, VOLUNTEER, DONOR, BENEFICIARY, STUDENT
//	STAFF       -> DONOR, BENEFICIARY, STUDENT
//	VOLUNTEER   -> DONOR, BENEFICIARY, STUDENT
//	anything else -> nothing (unauthorised)
func TestCanAssignRole(t *testing.T) {
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
			t.Run(actor+"->"+target, func(t *testing.T) {
				if got := CanAssignRole(actor, target); got != want {
					t.Errorf("CanAssignRole(%q, %q) = %v, want %v", actor, target, got, want)
				}
			})
		}
	}

	// Unknown or empty roles must never be allowed to create anything.
	for _, actor := range []string{"Ghost", "", "superadmin", " super admin "} {
		for _, target := range models.AllRoles() {
			if CanAssignRole(actor, target) {
				t.Errorf("CanAssignRole(%q, %q) = true; unknown roles must never create accounts", actor, target)
			}
		}
	}
}

// TestCanAssignRoleNoSelfOrEscalation pins the privilege-escalation guards:
// a role can never create an account above its own level. The one sanctioned
// same-level creation is Admin -> Admin (multi-Admin), asserted separately in
// TestAdminCanCreateAdmin.
func TestCanAssignRoleNoSelfOrEscalation(t *testing.T) {
	if CanAssignRole(models.RoleSuperAdmin, models.RoleSuperAdmin) {
		t.Error("Super Admin must not be able to create another Super Admin")
	}
	if CanAssignRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Error("Admin must not be able to create a Super Admin")
	}
	if CanAssignRole(models.RolePartner, models.RolePartner) {
		t.Error("Manager (Partner role) must not be able to create another Manager")
	}
	if CanAssignRole(models.RolePartner, models.RoleAdmin) {
		t.Error("Manager (Partner role) must not be able to create an Admin")
	}
	if CanAssignRole(models.RolePartner, models.RoleSuperAdmin) {
		t.Error("Manager (Partner role) must not be able to create a Super Admin")
	}
	// Manager (Partner role) MAY create Student accounts per the final role
	// model — covered positively by TestFinalCreationRules.
}

// TestFinalCreationRules pins the FINAL role-creation rules explicitly, one
// subtest per business rule (Part 1 of the role-model specification). The
// exhaustive matrix in TestCanAssignRole stays as the safety net; this test
// makes every individual rule auditable by name.
func TestFinalCreationRules(t *testing.T) {
	allowed := [][2]string{
		// Super Admin creates every NORMAL role...
		{models.RoleSuperAdmin, models.RoleAdmin},
		{models.RoleSuperAdmin, models.RolePartner},
		{models.RoleSuperAdmin, models.RoleStaff},
		{models.RoleSuperAdmin, models.RoleVolunteer},
		{models.RoleSuperAdmin, models.RoleDonor},
		{models.RoleSuperAdmin, models.RoleBeneficiary},
		{models.RoleSuperAdmin, models.RoleStudent},
		// ...but never another Super Admin.
		// Admin creates every normal NGO role...
		{models.RoleAdmin, models.RoleAdmin},
		{models.RoleAdmin, models.RolePartner},
		{models.RoleAdmin, models.RoleStaff},
		{models.RoleAdmin, models.RoleVolunteer},
		{models.RoleAdmin, models.RoleDonor},
		{models.RoleAdmin, models.RoleBeneficiary},
		{models.RoleAdmin, models.RoleStudent},
		// ...but never Super Admin. Admin→Admin/Manager need NO separate
		// admin.create / partner.create permission (hierarchy-only).
		// Manager (Partner role) creates lower operational roles only.
		{models.RolePartner, models.RoleStaff},
		{models.RolePartner, models.RoleVolunteer},
		{models.RolePartner, models.RoleDonor},
		{models.RolePartner, models.RoleBeneficiary},
		{models.RolePartner, models.RoleStudent},
		// Staff creates Donor / Beneficiary / Student.
		{models.RoleStaff, models.RoleDonor},
		{models.RoleStaff, models.RoleBeneficiary},
		{models.RoleStaff, models.RoleStudent},
		// Volunteer creates Donor / Beneficiary / Student.
		{models.RoleVolunteer, models.RoleDonor},
		{models.RoleVolunteer, models.RoleBeneficiary},
		{models.RoleVolunteer, models.RoleStudent},
	}

	for _, pair := range allowed {
		actor, target := pair[0], pair[1]
		t.Run(actor+" -> "+target+" allowed", func(t *testing.T) {
			if !CanAssignRole(actor, target) {
				t.Fatalf("CanAssignRole(%q, %q) = false, want true", actor, target)
			}
		})
	}

	blocked := [][2]string{
		{models.RoleSuperAdmin, models.RoleSuperAdmin}, // rule 8
		{models.RoleAdmin, models.RoleSuperAdmin},      // rule 16
		{models.RolePartner, models.RoleAdmin},         // rule 22
		{models.RolePartner, models.RoleSuperAdmin},    // rule 23
		{models.RolePartner, models.RolePartner},       // rule 24
		{models.RoleStaff, models.RoleSuperAdmin},
		{models.RoleStaff, models.RoleAdmin},
		{models.RoleStaff, models.RolePartner},
		{models.RoleStaff, models.RoleStaff},
		{models.RoleStaff, models.RoleVolunteer},
		{models.RoleVolunteer, models.RoleSuperAdmin},
		{models.RoleVolunteer, models.RoleAdmin},
		{models.RoleVolunteer, models.RolePartner},
		{models.RoleVolunteer, models.RoleVolunteer},
		{models.RoleDonor, models.RoleSuperAdmin},
		{models.RoleDonor, models.RoleAdmin},
		{models.RoleDonor, models.RoleDonor},
		{models.RoleBeneficiary, models.RoleSuperAdmin},
		{models.RoleBeneficiary, models.RoleAdmin},
		{models.RoleBeneficiary, models.RoleBeneficiary},
		{models.RoleStudent, models.RoleSuperAdmin},
		{models.RoleStudent, models.RoleAdmin},
		{models.RoleStudent, models.RoleStudent},
	}

	for _, pair := range blocked {
		actor, target := pair[0], pair[1]
		t.Run(actor+" -> "+target+" MUST FAIL", func(t *testing.T) {
			if CanAssignRole(actor, target) {
				t.Fatalf("CanAssignRole(%q, %q) = true, want false", actor, target)
			}
		})
	}
}

// TestAssignableRolesForMatchesCanAssignRole pins AssignableRolesFor (the
// list the frontend Add-User role selector is rendered from) to CanAssignRole
// (the backend enforcement): the dropdown can only ever offer roles the
// backend will accept, and never includes the actor's own privileged role
// unless same-level creation is sanctioned (Admin only).
func TestAssignableRolesForMatchesCanAssignRole(t *testing.T) {
	for _, actor := range models.AllRoles() {
		assignable := AssignableRolesFor(actor)
		if len(assignable) == 0 {
			for _, target := range models.AllRoles() {
				if CanAssignRole(actor, target) {
					t.Fatalf("actor %q can assign %q but AssignableRolesFor is empty", actor, target)
				}
			}
			continue
		}

		for _, target := range assignable {
			if !CanAssignRole(actor, target) {
				t.Errorf("AssignableRolesFor(%q) offers %q which CanAssignRole rejects", actor, target)
			}
		}

		if actor == models.RoleSuperAdmin || actor == models.RoleAdmin {
			// The Add-User dropdown must never offer Super Admin — not even
			// to the Super Admin (no Super Admin may mint another).
			for _, target := range assignable {
				if target == models.RoleSuperAdmin {
					t.Errorf("AssignableRolesFor(%q) must not offer Super Admin", actor)
				}
			}
		}
	}

	// Unknown actors get an empty (not nil) list so templates can range it.
	unknown := AssignableRolesFor("Ghost")
	if unknown == nil || len(unknown) != 0 {
		t.Errorf("AssignableRolesFor(unknown) = %v, want empty non-nil slice", unknown)
	}
}

// TestCanManageAccountRole verifies the account-management boundary:
//
//	Super Admin -> every role (including itself)
//	Admin       -> Admin, Partner, Staff, Volunteer, Donor, Beneficiary, Student
//	Partner     -> Staff, Volunteer, Donor, Beneficiary, Student
//	Staff       -> Donor, Beneficiary, Student
//	Volunteer   -> Donor, Beneficiary, Student
//	anything else -> nothing
//
// Admin accounts are managed by Super Admin and Admin (multi-Admin); Super
// Admin accounts can only ever be managed by a Super Admin.
func TestCanManageAccountRole(t *testing.T) {
	allowed := map[string]map[string]bool{
		models.RoleSuperAdmin: {
			models.RoleSuperAdmin:  true,
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
			t.Run(actor+" manages "+target, func(t *testing.T) {
				if got := CanManageAccountRole(actor, target); got != want {
					t.Errorf("CanManageAccountRole(%q, %q) = %v, want %v", actor, target, got, want)
				}
			})
		}
	}

	// Unknown roles must never be allowed to manage anything.
	for _, actor := range []string{"Ghost", ""} {
		for _, target := range models.AllRoles() {
			if CanManageAccountRole(actor, target) {
				t.Errorf("CanManageAccountRole(%q, %q) = true; unknown roles must never manage accounts", actor, target)
			}
		}
	}
}

// TestAssignableRolesSubsetOfManageableRoles keeps the two maps consistent:
// anything an actor may create it must also be allowed to manage.
func TestAssignableRolesSubsetOfManageableRoles(t *testing.T) {
	for actor, targets := range assignableRolesFor {
		for _, target := range targets {
			if !CanManageAccountRole(actor, target) {
				t.Errorf("role %q may assign %q but may not manage it", actor, target)
			}
		}
	}
}

// TestHierarchyMapsReferenceKnownRoles guards against typos in the
// single-source-of-truth hierarchy maps.
func TestHierarchyMapsReferenceKnownRoles(t *testing.T) {
	known := map[string]bool{}
	for _, role := range models.AllRoles() {
		known[role] = true
	}

	for actor, targets := range assignableRolesFor {
		if !known[actor] {
			t.Errorf("assignableRolesFor references unknown actor role %q", actor)
		}
		for _, target := range targets {
			if !known[target] {
				t.Errorf("assignableRolesFor[%q] references unknown target role %q", actor, target)
			}
		}
	}

	for actor, targets := range managedRolesFor {
		if !known[actor] {
			t.Errorf("managedRolesFor references unknown actor role %q", actor)
		}
		for _, target := range targets {
			if !known[target] {
				t.Errorf("managedRolesFor[%q] references unknown target role %q", actor, target)
			}
		}
	}
}

// TestIsProtectedPermission verifies that only the admin.*, super_admin.*,
// system.* and protected_rbac.* namespaces are treated as protected.
func TestIsProtectedPermission(t *testing.T) {
	protected := []string{
		"admin.view",
		"admin.create",
		// The Admin deletion workflow permissions remain admin.* and thus
		// protected: they are held by the Super Admin and Admin roles only
		// and can never be granted to lower roles.
		"admin.delete.request",
		"admin.delete.approve",
		"admin.delete.reject",
		"super_admin.impersonate",
		"system.reset",
		"protected_rbac.manage",
		// Case-insensitive and whitespace-tolerant.
		"ADMIN.VIEW",
		"  admin.view  ",
		"Protected_RBAC.Manage",
	}

	for _, name := range protected {
		if !IsProtectedPermission(name) {
			t.Errorf("IsProtectedPermission(%q) = false, want true", name)
		}
	}

	notProtected := []string{
		"",
		"users.view",
		"partner.create",
		"donor.edit",
		"beneficiary.delete",
		"reports.view",
		"audit_logs.view",
		// Similar-looking names that must not match the namespaces.
		"admin",
		"administrator.view",
		"superadmin.view",
		"systematic.view",
		"protectedrbac.manage",
	}

	for _, name := range notProtected {
		if IsProtectedPermission(name) {
			t.Errorf("IsProtectedPermission(%q) = true, want false", name)
		}
	}
}

// TestFilterProtectedPermissions verifies the allowed/blocked split.
func TestFilterProtectedPermissions(t *testing.T) {
	input := []string{
		"users.view",
		"admin.view",
		"donation.edit",
		"SYSTEM.RESET",
		"partner.create",
		"protected_rbac.manage",
	}

	allowed, blocked := FilterProtectedPermissions(input)

	wantAllowed := []string{"users.view", "donation.edit", "partner.create"}
	wantBlocked := []string{"admin.view", "SYSTEM.RESET", "protected_rbac.manage"}

	if !reflect.DeepEqual(allowed, wantAllowed) {
		t.Errorf("allowed = %v, want %v", allowed, wantAllowed)
	}
	if !reflect.DeepEqual(blocked, wantBlocked) {
		t.Errorf("blocked = %v, want %v", blocked, wantBlocked)
	}

	// Empty input yields empty, non-nil slices.
	allowed, blocked = FilterProtectedPermissions([]string{})
	if len(allowed) != 0 || allowed == nil {
		t.Errorf("allowed = %v, want empty non-nil slice", allowed)
	}
	if len(blocked) != 0 || blocked == nil {
		t.Errorf("blocked = %v, want empty non-nil slice", blocked)
	}

	// Everything allowed.
	allowed, blocked = FilterProtectedPermissions([]string{"users.view", "donation.edit"})
	if !reflect.DeepEqual(allowed, []string{"users.view", "donation.edit"}) || len(blocked) != 0 {
		t.Errorf("unexpected split: allowed=%v blocked=%v", allowed, blocked)
	}

	// Everything blocked.
	allowed, blocked = FilterProtectedPermissions([]string{"admin.view", "system.x"})
	if len(allowed) != 0 || !reflect.DeepEqual(blocked, []string{"admin.view", "system.x"}) {
		t.Errorf("unexpected split: allowed=%v blocked=%v", allowed, blocked)
	}
}

// TestRoleMayHoldPermission verifies the role-aware protected-permission
// rule: Super Admin may hold every permission; the Admin role may hold
// admin.* (multi-Admin) but no other protected namespace; every other role
// may hold no protected permission at all.
func TestRoleMayHoldPermission(t *testing.T) {
	adminPerms := []string{
		"admin.view", "admin.create", "admin.edit",
		"admin.activate", "admin.deactivate",
		"admin.delete.request", "admin.delete.approve", "admin.delete.reject",
	}
	otherProtected := []string{
		"super_admin.impersonate", "system.settings.edit", "protected_rbac.manage",
	}

	// Super Admin may hold everything.
	for _, name := range append(append([]string{}, adminPerms...), otherProtected...) {
		if !RoleMayHoldPermission(models.RoleSuperAdmin, name) {
			t.Errorf("Super Admin must be able to hold %q", name)
		}
	}

	// Admin may hold admin.* but nothing else protected.
	for _, name := range adminPerms {
		if !RoleMayHoldPermission(models.RoleAdmin, name) {
			t.Errorf("Admin must be able to hold admin.* permission %q (multi-Admin)", name)
		}
	}
	for _, name := range otherProtected {
		if RoleMayHoldPermission(models.RoleAdmin, name) {
			t.Errorf("Admin must NOT be able to hold %q", name)
		}
	}

	// Non-admin operational roles may hold no protected permission.
	for _, actor := range []string{
		models.RolePartner, models.RoleStaff, models.RoleVolunteer,
		models.RoleDonor, models.RoleBeneficiary, models.RoleStudent,
	} {
		for _, name := range append(append([]string{}, adminPerms...), otherProtected...) {
			if RoleMayHoldPermission(actor, name) {
				t.Errorf("%q must NOT be able to hold protected permission %q", actor, name)
			}
		}
		// Unprotected permissions remain assignable to every role.
		if !RoleMayHoldPermission(actor, "users.view") {
			t.Errorf("%q must be able to hold unprotected permission users.view", actor)
		}
	}
}

// TestFilterPermissionsForRole verifies the role-aware allowed/blocked split:
// admin.* is allowed for the Admin and Super Admin roles and blocked for every
// other role.
func TestFilterPermissionsForRole(t *testing.T) {
	input := []string{"users.view", "admin.view", "admin.create", "system.reset", "donation.edit"}

	// Admin role: admin.* allowed, super_admin/system.* blocked.
	allowed, blocked := filterPermissionsForRole(models.RoleAdmin, input)
	if !reflect.DeepEqual(allowed, []string{"users.view", "admin.view", "admin.create", "donation.edit"}) {
		t.Errorf("Admin role allowed = %v", allowed)
	}
	if !reflect.DeepEqual(blocked, []string{"system.reset"}) {
		t.Errorf("Admin role blocked = %v", blocked)
	}

	// Manager (Partner role): every protected permission blocked.
	allowed, blocked = filterPermissionsForRole(models.RolePartner, input)
	if !reflect.DeepEqual(allowed, []string{"users.view", "donation.edit"}) {
		t.Errorf("Manager (Partner role) allowed = %v", allowed)
	}
	if !reflect.DeepEqual(blocked, []string{"admin.view", "admin.create", "system.reset"}) {
		t.Errorf("Manager (Partner role) blocked = %v", blocked)
	}

	// Super Admin: everything allowed.
	allowed, blocked = filterPermissionsForRole(models.RoleSuperAdmin, input)
	if len(blocked) != 0 || !reflect.DeepEqual(allowed, input) {
		t.Errorf("Super Admin role allowed=%v blocked=%v", allowed, blocked)
	}
}
