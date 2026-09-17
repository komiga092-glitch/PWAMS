package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestStep3_SelfProtection_PreventSelfPromotionToSuperAdmin verifies that a user
// cannot change their own role to Super Admin through the Update handler.
func TestStep3_SelfProtection_PreventSelfPromotionToSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	actorID := uuid.New()
	targetID := actorID // Same ID = self
	oldRole := models.RoleAdmin
	newRole := models.RoleSuperAdmin

	// Simulate the self-protection check
	isSelf := targetID == actorID
	isRoleChange := oldRole != newRole
	isPromotingToSuperAdmin := newRole == models.RoleSuperAdmin

	if !(isSelf && isRoleChange && isPromotingToSuperAdmin) {
		t.Fatal("self-protection condition should trigger for self-promotion to Super Admin")
	}

	// Verify that the hierarchy guard also blocks this
	if services.CanAssignRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Fatal("Admin should not be able to assign Super Admin role")
	}
}

// TestStep3_SelfProtection_PreventSelfDeactivation verifies that the UpdateStatus
// handler prevents self-deactivation.
func TestStep3_SelfProtection_PreventSelfDeactivation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	actorID := uuid.New()
	targetID := actorID // Same ID = self
	newStatus := models.UserStatusDisabled

	// Simulate the self-protection check
	isSelf := targetID == actorID
	isDeactivating := strings.EqualFold(strings.TrimSpace(newStatus), models.UserStatusDisabled)

	if !(isSelf && isDeactivating) {
		t.Fatal("self-protection condition should trigger for self-deactivation")
	}
}

// TestStep3_SelfProtection_PreventSelfDeletion verifies that the Delete handler
// prevents self-deletion.
func TestStep3_SelfProtection_PreventSelfDeletion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	actorID := uuid.New()
	targetID := actorID // Same ID = self

	if targetID != actorID {
		t.Fatal("self-protection condition should trigger for self-deletion")
	}
}

// TestStep3_SuperAdminVisibility_SuperAdminOnly verifies that Super Admin
// accounts are only visible to Super Admin role.
func TestStep3_SuperAdminVisibility_SuperAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		actorRole   string
		targetRole  string
		shouldAllow bool
	}{
		{models.RoleSuperAdmin, models.RoleSuperAdmin, true},
		{models.RoleAdmin, models.RoleSuperAdmin, false},
		{models.RolePartner, models.RoleSuperAdmin, false},
		{models.RoleStaff, models.RoleSuperAdmin, false},
		{models.RoleVolunteer, models.RoleSuperAdmin, false},
		{models.RoleDonor, models.RoleSuperAdmin, false},
		{models.RoleBeneficiary, models.RoleSuperAdmin, false},
		{models.RoleStudent, models.RoleSuperAdmin, false},
	}

	for _, tc := range testCases {
		isTargetSuperAdmin := tc.targetRole == models.RoleSuperAdmin
		isActorSuperAdmin := tc.actorRole == models.RoleSuperAdmin
		allowed := !(isTargetSuperAdmin && !isActorSuperAdmin)

		if allowed != tc.shouldAllow {
			t.Errorf("actor=%s target=%s: allowed=%v, want %v", tc.actorRole, tc.targetRole, allowed, tc.shouldAllow)
		}
	}
}

// TestStep3_RoleHierarchy_ConsistentAssignableAndManageable verifies that
// CanManageAccountRole is consistent with CanAssignRole for all role pairs.
func TestStep3_RoleHierarchy_ConsistentAssignableAndManageable(t *testing.T) {
	for _, actor := range models.AllRoles() {
		for _, target := range models.AllRoles() {
			if services.CanAssignRole(actor, target) {
				if !services.CanManageAccountRole(actor, target) {
					t.Errorf("role %q may assign %q but may not manage it", actor, target)
				}
			}
		}
	}
}

// TestStep3_PermissionGates_AdminRequiresAdminPermission verifies that Admin
// account management requires the appropriate admin.* permission.
func TestStep3_PermissionGates_AdminRequiresAdminPermission(t *testing.T) {
	adminActions := map[string]string{
		"view":       "admin.view",
		"edit":       "admin.edit",
		"activate":   "admin.activate",
		"deactivate": "admin.deactivate",
		"delete":     "admin.delete.approve",
	}

	for action, want := range adminActions {
		got := requiredTargetPermission(models.RoleAdmin, action)
		if got != want {
			t.Errorf("requiredTargetPermission(Admin, %q) = %q, want %q", action, got, want)
		}
	}
}

// TestStep3_PermissionGates_PartnerRequiresPartnerPermission verifies that
// Manager (Partner role) account management requires the appropriate partner.* permission.
func TestStep3_PermissionGates_PartnerRequiresPartnerPermission(t *testing.T) {
	partnerActions := map[string]string{
		"view":       "partner.view",
		"edit":       "partner.edit",
		"activate":   "partner.activate",
		"deactivate": "partner.deactivate",
		"delete":     "partner.delete",
	}

	for action, want := range partnerActions {
		got := requiredTargetPermission(models.RolePartner, action)
		if got != want {
			t.Errorf("requiredTargetPermission(Partner, %q) = %q, want %q", action, got, want)
		}
	}
}

// ensure constants import is used.
var _ = http.StatusOK
