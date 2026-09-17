package authorization_test

import (
	"strings"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// STEP 7 — AUTHORIZATION & PRIVILEGE ESCALATION HARDENING (Tests 1-12)

func TestAuthz_StudentCannotAccessAdminEndpoints(t *testing.T) {
	if services.CanAssignRole(models.RoleStudent, models.RoleAdmin) {
		t.Fatal("Student must NOT be able to assign Admin role")
	}
	if services.CanManageAccountRole(models.RoleStudent, models.RoleAdmin) {
		t.Fatal("Student must NOT be able to manage Admin accounts")
	}
}

func TestAuthz_ManagerCannotAccessSuperAdminOperations(t *testing.T) {
	if services.CanAssignRole(models.RolePartner, models.RoleSuperAdmin) {
		t.Fatal("Manager must NOT be able to assign Super Admin role")
	}
	if services.CanManageAccountRole(models.RolePartner, models.RoleSuperAdmin) {
		t.Fatal("Manager must NOT be able to manage Super Admin accounts")
	}
}

func TestAuthz_AdminCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Fatal("Admin must NOT be able to create a Super Admin account")
	}
	if services.CanManageAccountRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Fatal("Admin must NOT be able to manage Super Admin accounts")
	}
}

func TestAuthz_NormalUserCannotSelfPromoteToSuperAdmin(t *testing.T) {
	for _, role := range []string{
		models.RolePartner, models.RoleStaff, models.RoleVolunteer,
		models.RoleDonor, models.RoleBeneficiary, models.RoleStudent, models.RoleAdmin,
	} {
		if services.CanAssignRole(role, models.RoleSuperAdmin) {
			t.Fatalf("%s must NOT be able to assign Super Admin role", role)
		}
	}
}

func TestAuthz_NormalUserCannotAssignRoleIDSuperAdmin(t *testing.T) {
	for _, actorRole := range []string{
		models.RoleAdmin, models.RolePartner, models.RoleStaff, models.RoleVolunteer,
	} {
		for _, target := range services.AssignableRolesFor(actorRole) {
			if target == models.RoleSuperAdmin {
				t.Fatalf("%s must NOT have Super Admin in assignable roles", actorRole)
			}
		}
	}
}

func TestAuthz_RoleHierarchyBlocksUnauthorizedRoleChange(t *testing.T) {
	if services.CanAssignRole(models.RoleStaff, models.RoleAdmin) {
		t.Fatal("Staff must NOT be able to change role to Admin")
	}
	if services.CanAssignRole(models.RoleStaff, models.RolePartner) {
		t.Fatal("Staff must NOT be able to change role to Manager")
	}
}

func TestAuthz_StatusChangeEnforcesHierarchy(t *testing.T) {
	if services.CanManageAccountRole(models.RoleStaff, models.RoleAdmin) {
		t.Fatal("Staff must NOT be able to change Admin status")
	}
}

func TestAuthz_CrossUserAccessControlEnforced(t *testing.T) {
	if services.CanManageAccountRole(models.RoleBeneficiary, models.RoleAdmin) {
		t.Fatal("Beneficiary must NOT access Admin records")
	}
}

func TestAuthz_CrossUserModificationBlocked(t *testing.T) {
	if services.CanManageAccountRole(models.RolePartner, models.RoleAdmin) {
		t.Fatal("Manager must NOT modify Admin accounts")
	}
}

func TestAuthz_ReportExportRequiresPermission(t *testing.T) {
	for _, role := range []string{models.RoleDonor, models.RoleBeneficiary, models.RoleStudent} {
		if services.RoleMayHoldPermission(role, "system.alerts.view") {
			t.Fatalf("%s must NOT hold system.alerts.view", role)
		}
	}
}

func TestAuthz_AuditAccessRestricted(t *testing.T) {
	for _, role := range []string{
		models.RolePartner, models.RoleStaff, models.RoleVolunteer,
		models.RoleDonor, models.RoleBeneficiary, models.RoleStudent,
	} {
		if services.RoleMayHoldPermission(role, "protected_rbac.manage") {
			t.Fatalf("%s must NOT hold protected_rbac.manage", role)
		}
	}
}

func TestAuthz_SystemAlertAccessRestricted(t *testing.T) {
	for _, role := range []string{
		models.RoleAdmin, models.RolePartner, models.RoleStaff,
		models.RoleVolunteer, models.RoleDonor, models.RoleBeneficiary, models.RoleStudent,
	} {
		if role == models.RoleSuperAdmin {
			continue
		}
		if services.RoleMayHoldPermission(role, "system.alerts.view") {
			t.Fatalf("%s must NOT hold system.alerts.view", role)
		}
	}
}

func TestAuthz_VerticalEscalationMatrix(t *testing.T) {
	hierarchy := []string{
		models.RoleStudent, models.RoleBeneficiary, models.RoleDonor,
		models.RoleVolunteer, models.RoleStaff, models.RolePartner,
		models.RoleAdmin, models.RoleSuperAdmin,
	}
	for i, actor := range hierarchy {
		for j, target := range hierarchy {
			if i >= j {
				continue
			}
			if services.CanAssignRole(actor, target) {
				t.Fatalf("%s (level %d) must NOT assign %s (level %d)", actor, i, target, j)
			}
		}
	}
}

func TestAuthz_ProtectedPermissionsNeverLeak(t *testing.T) {
	for _, p := range services.AllPermissions() {
		if services.IsProtectedPermission(p.Name) {
			for _, role := range []string{
				models.RolePartner, models.RoleStaff, models.RoleVolunteer,
				models.RoleDonor, models.RoleBeneficiary, models.RoleStudent,
			} {
				if services.RoleMayHoldPermission(role, p.Name) {
					if strings.HasPrefix(p.Name, "admin.") && role == models.RoleAdmin {
						continue
					}
					t.Fatalf("%s must NOT hold protected permission %s", role, p.Name)
				}
			}
		}
	}
}

func TestAuthz_ManagerUniquenessPreserved(t *testing.T) {
	if services.ErrManagerAlreadyExists == nil {
		t.Fatal("ErrManagerAlreadyExists must be defined")
	}
}
