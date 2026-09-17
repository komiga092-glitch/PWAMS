package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// PermissionHandler exposes the permission-management endpoints used by
// authorized Admin/Super Admin users to inspect and modify role defaults and
// user-specific permission overrides.
type PermissionHandler struct {
	permissionService *services.PermissionService
	userService       *services.UserService
}

func NewPermissionHandler(
	permissionService *services.PermissionService,
	userService *services.UserService,
) *PermissionHandler {
	return &PermissionHandler{
		permissionService: permissionService,
		userService:       userService,
	}
}

// Page renders the permission-management HTML page (used by authorized
// admin/super-admin users to edit role permission defaults).
func (h *PermissionHandler) Page(c *gin.Context) {
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "permissions_content",
		"title":         "Permissions - PWAMS",
	}))
}

// ListAll returns the canonical permission catalog grouped by category.
//
// Protected permissions (admin.*, super_admin.*, system.*,
// protected_rbac.*) are only ever included for Super Admin actors. The
// permission designer must not display protected permissions to any other
// actor, so they are filtered out server-side — the client never even
// receives their names (spec: protected admin permissions).
func (h *PermissionHandler) ListAll(c *gin.Context) {
	perms := services.AllPermissions()

	if !isSuperAdminActor(c) {
		perms = filterCatalogForNonSuperAdmin(perms)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"permissions": perms,
	})
}

// designerRoles lists the roles an Admin may configure in the permission
// designer: the operational roles only. Super Admin and Admin role sets are
// protected and can never be modified through the designer by a non-Super
// Admin.
var designerRoles = map[string]bool{
	models.RolePartner:   true,
	models.RoleStaff:     true,
	models.RoleVolunteer: true,
}

// ListRoles returns every role with its currently assigned permission names.
// Non-Super Admin actors only receive the operational roles (Partner, Staff,
// Volunteer), matching the service-layer guard that rejects modifications to
// the protected roles.
func (h *PermissionHandler) ListRoles(c *gin.Context) {
	roles, err := h.permissionService.ListRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve roles"})
		return
	}

	superAdmin := isSuperAdminActor(c)

	result := make([]gin.H, 0, len(roles))
	for _, role := range roles {
		if !superAdmin && !designerRoles[role.Name] {
			continue
		}

		perms, err := h.permissionService.GetRolePermissions(role.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve role permissions"})
			return
		}
		result = append(result, gin.H{
			"id":          role.ID,
			"name":        role.Name,
			"description": role.Description,
			"permissions": perms,
			"created_at":  role.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"roles":   result,
	})
}

// GetRolePermissions returns the permission names for a single role.
// Non-Super Admin actors may only inspect the operational roles; requesting
// a protected role's permission set (which contains protected permission
// names) is rejected.
func (h *PermissionHandler) GetRolePermissions(c *gin.Context) {
	roleName := c.Param("name")
	if roleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Role name is required"})
		return
	}

	if !isSuperAdminActor(c) && !designerRoles[roleName] {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "This role's permission set can only be viewed by a Super Admin",
		})
		return
	}

	perms, err := h.permissionService.GetRolePermissions(roleName)
	if err != nil {
		if errors.Is(err, services.ErrRoleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve role permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"role":        roleName,
		"permissions": perms,
	})
}

type updateRolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

// UpdateRolePermissions replaces a role's permission set.
func (h *PermissionHandler) UpdateRolePermissions(c *gin.Context) {
	roleName := c.Param("name")
	if roleName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Role name is required"})
		return
	}

	var request updateRolePermissionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid role permissions payload"})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	// Server-side authorization: the actor must be permitted to manage this
	// role's permissions (privilege-escalation guard).
	if err := h.permissionService.SetRolePermissionsByActor(currentUser, roleName, request.Permissions); err != nil {
		switch {
		case errors.Is(err, services.ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrProtectedPermission):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrRoleNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to update role permissions"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Role permissions updated",
	})
}

// GetUserOverrides returns a user's explicit permission overrides.
func (h *PermissionHandler) GetUserOverrides(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID"})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	target, err := h.userService.GetUserByID(userID.String())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve user"})
		return
	}

	if err := h.permissionService.CanManageUserPermissions(currentUser, target); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		return
	}

	overrides, err := h.permissionService.GetUserPermissionOverrides(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve user permission overrides"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"user":      target,
		"overrides": overrides,
	})
}

type updateUserOverridesRequest struct {
	Overrides []services.UserPermissionOverride `json:"overrides"`
}

// UpdateUserOverrides applies explicit grants/revocations for a specific user.
func (h *PermissionHandler) UpdateUserOverrides(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID"})
		return
	}

	var request updateUserOverridesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid permission overrides payload"})
		return
	}

	actor, ok := getCurrentUser(c)
	if !ok {
		return
	}

	target, err := h.userService.GetUserByID(userID.String())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve user"})
		return
	}

	if err := h.permissionService.ApplyUserPermissionOverrides(actor, target, request.Overrides); err != nil {
		switch {
		case errors.Is(err, services.ErrPermissionDenied):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrProtectedPermission):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrPermissionNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to apply permission overrides"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User permission overrides updated",
	})
}
