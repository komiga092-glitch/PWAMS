package services

import (
	"strings"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// ---------------------------------------------------------------------------
// Super Admin -> Admin management (spec section 34: Super Admin)
// ---------------------------------------------------------------------------

func TestSuperAdminCanCreateAdmin(t *testing.T) {
	if !CanAssignRole(models.RoleSuperAdmin, models.RoleAdmin) {
		t.Fatal("Super Admin must be able to create Admin accounts")
	}
}

func TestSuperAdminCanEditAdmin(t *testing.T) {
	if !CanManageAccountRole(models.RoleSuperAdmin, models.RoleAdmin) {
		t.Fatal("Super Admin must be able to edit Admin accounts")
	}
}

func TestSuperAdminCanActivateAdmin(t *testing.T) {
	if !CanManageAccountRole(models.RoleSuperAdmin, models.RoleAdmin) {
		t.Fatal("Super Admin must be able to activate Admin accounts")
	}
}

func TestSuperAdminCanDeactivateAdmin(t *testing.T) {
	if !CanManageAccountRole(models.RoleSuperAdmin, models.RoleAdmin) {
		t.Fatal("Super Admin must be able to deactivate Admin accounts")
	}
}

func TestSuperAdminCanDeleteAdmin(t *testing.T) {
	if !CanManageAccountRole(models.RoleSuperAdmin, models.RoleAdmin) {
		t.Fatal("Super Admin must be able to delete Admin accounts")
	}
}

// ---------------------------------------------------------------------------
// Admin -> Admin / Partner / Staff / Volunteer management (spec section 34:
// Admin + multi-Admin task: Admin may create and manage other Admins)
// ---------------------------------------------------------------------------

func TestAdminCanCreatePartner(t *testing.T) {
	if !CanAssignRole(models.RoleAdmin, models.RolePartner) {
		t.Fatal("Admin must be able to create Manager (Partner role) accounts")
	}
}

func TestAdminCanCreateStaff(t *testing.T) {
	if !CanAssignRole(models.RoleAdmin, models.RoleStaff) {
		t.Fatal("Admin must be able to create Staff accounts")
	}
}

func TestAdminCanCreateVolunteer(t *testing.T) {
	if !CanAssignRole(models.RoleAdmin, models.RoleVolunteer) {
		t.Fatal("Admin must be able to create Volunteer accounts")
	}
}

func TestAdminCanCreateAdmin(t *testing.T) {
	if !CanAssignRole(models.RoleAdmin, models.RoleAdmin) {
		t.Fatal("Admin must be able to create another Admin account (multi-Admin)")
	}
	if !CanManageAccountRole(models.RoleAdmin, models.RoleAdmin) {
		t.Fatal("Admin must be able to manage other Admin accounts (multi-Admin)")
	}
}

func TestAdminCannotCreateSuperAdmin(t *testing.T) {
	if CanAssignRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Fatal("Admin must NOT be able to create Super Admin accounts")
	}
}

func TestAdminCannotManageSuperAdmin(t *testing.T) {
	if CanManageAccountRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Fatal("Admin must NOT be able to manage Super Admin accounts")
	}
}

func TestAdminCanDeleteAdmin(t *testing.T) {
	if !CanManageAccountRole(models.RoleAdmin, models.RoleAdmin) {
		t.Fatal("Admin must be able to delete other Admin accounts (multi-Admin)")
	}
}

// ---------------------------------------------------------------------------
// Manager boundaries (spec section 34: Manager, backed by Partner role)
// ---------------------------------------------------------------------------

// TestPartnerHasFullOperationalAccess pins the default permission matrix:
// every newly created Manager (Partner role) inherits the full NGO operational permission
// set from its role (spec section 12) but never admin management, super
// admin management, system configuration, protected RBAC or the permission
// designer itself.
func TestPartnerHasFullOperationalAccess(t *testing.T) {
	defaults := defaultRolePermissions()
	partner := defaults[models.RolePartner]
	if len(partner) == 0 {
		t.Fatal("Manager (Partner role) default permission set must not be empty")
	}

	// Full NGO operational access (spec section 12 module list).
	operational := []string{
		// Staff / Volunteers
		"staff.view", "staff.create", "staff.edit", "staff.delete",
		"volunteer.view", "volunteer.create", "volunteer.edit",
		// Donor / Beneficiary / Student
		"donor.view", "donor.create", "donor.edit", "donor.delete",
		"beneficiary.view", "beneficiary.create", "beneficiary.edit", "beneficiary.delete",
		"student.view", "student.create", "student.edit", "student.delete",
		// Donations / Aid / Loans / Repayments / Care
		"donation.view", "donation.create", "donation.edit", "donation.delete",
		"aid.view", "aid.create", "aid.edit", "aid.approve",
		"loan.view", "loan.create", "loan.edit", "loan.approve",
		"repayment.view", "repayment.create", "repayment.edit",
		"care.view", "care.create", "care.edit", "care.delete",
		// Files / Messages / Notifications / Reports
		"file.view", "file.upload", "file.delete",
		"message.view", "message.send", "message.delete",
		"notification.view", "reports.view",
	}

	held := make(map[string]bool, len(partner))
	for _, name := range partner {
		held[name] = true
	}

	for _, perm := range operational {
		if !held[perm] {
			t.Errorf("Manager (Partner role) default set must include %q (full NGO operational access)", perm)
		}
	}

	// Never admin management / super admin / system config / protected RBAC /
	// permission designer (spec sections 12 and 20).
	for _, name := range partner {
		if IsProtectedPermission(name) {
			t.Errorf("Manager (Partner role) default set must not contain protected permission %q", name)
		}
		if strings.HasPrefix(name, "permission_management.") {
			t.Errorf("Manager (Partner role) default set must not contain %q", name)
		}
	}
}

func TestPartnerCannotCreateAdmin(t *testing.T) {
	if CanAssignRole(models.RolePartner, models.RoleAdmin) {
		t.Fatal("Manager (Partner role) must NOT be able to create Admin accounts")
	}
}

func TestPartnerCannotDeleteAdmin(t *testing.T) {
	if CanManageAccountRole(models.RolePartner, models.RoleAdmin) {
		t.Fatal("Manager (Partner role) must NOT be able to delete Admin accounts")
	}
}

func TestPartnerCannotModifySuperAdmin(t *testing.T) {
	if CanManageAccountRole(models.RolePartner, models.RoleSuperAdmin) {
		t.Fatal("Manager (Partner role) must NOT be able to modify Super Admin accounts")
	}
}

func TestPartnerCanManageStaffAndVolunteers(t *testing.T) {
	for _, target := range []string{models.RoleStaff, models.RoleVolunteer} {
		if !CanManageAccountRole(models.RolePartner, target) {
			t.Errorf("Manager (Partner role) must be able to manage %s accounts", target)
		}
	}
}

// ---------------------------------------------------------------------------
// Staff / Volunteer / Donor / Beneficiary / Student cannot create Admin
// ---------------------------------------------------------------------------

func TestStaffCannotCreateAdmin(t *testing.T) {
	if CanAssignRole(models.RoleStaff, models.RoleAdmin) {
		t.Fatal("Staff must NOT be able to create Admin accounts")
	}
}

func TestVolunteerCannotCreateAdmin(t *testing.T) {
	if CanAssignRole(models.RoleVolunteer, models.RoleAdmin) {
		t.Fatal("Volunteer must NOT be able to create Admin accounts")
	}
}

func TestDonorCannotCreateAdmin(t *testing.T) {
	if CanAssignRole(models.RoleDonor, models.RoleAdmin) {
		t.Fatal("Donor must NOT be able to create Admin accounts")
	}
}

func TestBeneficiaryCannotCreateAdmin(t *testing.T) {
	if CanAssignRole(models.RoleBeneficiary, models.RoleAdmin) {
		t.Fatal("Beneficiary must NOT be able to create Admin accounts")
	}
}

func TestStudentCannotCreateAdmin(t *testing.T) {
	if CanAssignRole(models.RoleStudent, models.RoleAdmin) {
		t.Fatal("Student must NOT be able to create Admin accounts")
	}
}

// TestAdminDefaultHoldsAdminPermissions pins that the Admin role's default
// permission set includes the full admin.* namespace (multi-Admin support)
// and never any super_admin.* / system.* / protected_rbac.* permission.
func TestAdminDefaultHoldsAdminPermissions(t *testing.T) {
	defaults := defaultRolePermissions()
	admin := defaults[models.RoleAdmin]

	held := make(map[string]bool, len(admin))
	for _, name := range admin {
		held[name] = true
	}

	for _, perm := range []string{
		"admin.view", "admin.create", "admin.edit",
		"admin.activate", "admin.deactivate",
		"admin.delete.request", "admin.delete.approve", "admin.delete.reject",
	} {
		if !held[perm] {
			t.Errorf("Admin default set must include %q (multi-Admin support)", perm)
		}
	}

	// The legacy monolithic delete permission must not come back: Admin
	// account deletion is a request/approve/reject workflow.
	if held["admin.delete"] {
		t.Errorf("Admin default set must not contain the removed legacy permission %q", "admin.delete")
	}

	for _, name := range admin {
		switch {
		case strings.HasPrefix(name, "super_admin."):
			t.Errorf("Admin default set must not contain %q", name)
		case strings.HasPrefix(name, "system."):
			t.Errorf("Admin default set must not contain %q", name)
		case strings.HasPrefix(name, "protected_rbac."):
			t.Errorf("Admin default set must not contain %q", name)
		}
	}
}
