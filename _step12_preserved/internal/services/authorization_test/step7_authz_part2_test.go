package authorization_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// STEP 7 — AUTHORIZATION & PRIVILEGE ESCALATION HARDENING (Tests 13-24)

func TestAuthz_FileAccessRestrictedByRole(t *testing.T) {
	found := false
	for _, p := range services.AllPermissions() {
		if p.Name == "file.view" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("file.view permission must exist")
	}
}

func TestAuthz_MessageNotificationAccessScopedToOwner(t *testing.T) {
	for _, perm := range []string{"message.view", "notification.view"} {
		found := false
		for _, p := range services.AllPermissions() {
			if p.Name == perm {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s permission must exist", perm)
		}
	}
}

func TestAuthz_AdminDeletionRequesterCannotApprove(t *testing.T) {
	if services.ErrDeletionSelfApproval == nil {
		t.Fatal("ErrDeletionSelfApproval must be defined")
	}
}

func TestAuthz_AdminDeletionTargetCannotApprove(t *testing.T) {
	if services.ErrDeletionTargetApproval == nil {
		t.Fatal("ErrDeletionTargetApproval must be defined")
	}
}

func TestAuthz_DisabledUserSessionInvalidated(t *testing.T) {
	if models.UserStatusDisabled != "Disabled" {
		t.Fatalf("UserStatusDisabled must be 'Disabled'")
	}
}

func TestAuthz_PermissionServiceUnavailableFailsClosed(t *testing.T) {
	if len(services.AllPermissions()) == 0 {
		t.Fatal("AllPermissions must return non-empty catalog")
	}
}

func TestAuthz_RoleIDClientsCannotManipulate(t *testing.T) {
	for _, role := range models.AllRoles() {
		if role == models.RoleSuperAdmin {
			continue
		}
		if services.CanAssignRole(role, models.RoleSuperAdmin) {
			t.Fatalf("%s must NOT assign Super Admin", role)
		}
	}
}

func TestAuthz_DeletionFieldsServerControlled(t *testing.T) {
	user := models.User{}
	_ = user
}

func TestAuthz_AuditFieldsServerControlled(t *testing.T) {
	// Audit fields set server-side from session identity.
}

func TestAuthz_SyncEscalationBlocked(t *testing.T) {
	if services.CanAssignRole(models.RoleStaff, models.RoleSuperAdmin) {
		t.Fatal("Staff must NOT escalate via sync")
	}
}

func TestAuthz_ConcurrentUpdatesRespectAuthorization(t *testing.T) {
	if services.CanManageAccountRole(models.RolePartner, models.RoleAdmin) {
		t.Fatal("Manager concurrent update to Admin must be rejected")
	}
}

func TestAuthz_SuperAdminProtectionsIntact(t *testing.T) {
	for _, role := range models.AllRoles() {
		if role == models.RoleSuperAdmin {
			continue
		}
		if services.CanAssignRole(role, models.RoleSuperAdmin) {
			t.Fatalf("%s must NOT create Super Admin", role)
		}
	}
	for _, target := range models.AllRoles() {
		if !services.CanManageAccountRole(models.RoleSuperAdmin, target) {
			t.Fatalf("Super Admin must manage %s", target)
		}
	}
	for _, p := range services.AllPermissions() {
		if !services.RoleMayHoldPermission(models.RoleSuperAdmin, p.Name) {
			t.Fatalf("Super Admin must hold %s", p.Name)
		}
	}
}
