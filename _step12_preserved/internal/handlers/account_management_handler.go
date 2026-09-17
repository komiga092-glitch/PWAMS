package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// ManagedAccountConfig describes one role-scoped account-management module
// (Admin Management, Manager Management, Staff Management, Volunteer
// Management). Every module shares one handler implementation; the target
// role is fixed by the configuration and is therefore always decided
// server-side — the client can never influence which role gets created or
// edited.
type ManagedAccountConfig struct {
	// Role is the account role managed by this module.
	Role string
	// Label is the human-facing module name used in page headings.
	Label string
	// APIBase is the URL prefix of the module, e.g. "/admins".
	APIBase string
	// Deletable reports whether the module exposes a delete action. It is
	// only true for roles whose catalog defines a delete permission
	// (admin.delete.approve, partner.delete, staff.delete). Volunteers have
	// no delete permission by design; deactivation is the supported removal.
	// For the Admin module, deletion is additionally available through the
	// supervised /admins/deletion-requests request/approve/reject workflow.
	Deletable bool
}

// AccountManagementHandler implements the role-scoped management endpoints
// backed by the existing UserService. It reuses the account-management
// guards from user_handler.go (canCreateRole, hasTargetPermission,
// requiredTargetPermission) so there is a single authorization
// implementation shared by the generic and scoped endpoints.
type AccountManagementHandler struct {
	config          ManagedAccountConfig
	userService     *services.UserService
	auditLogService *services.AuditLogService
}

// NewAccountManagementHandler builds the handler for one managed role.
func NewAccountManagementHandler(
	config ManagedAccountConfig,
	userService *services.UserService,
	auditLogService *services.AuditLogService,
) *AccountManagementHandler {
	return &AccountManagementHandler{
		config:          config,
		userService:     userService,
		auditLogService: auditLogService,
	}
}

// Config exposes the module configuration (used by route registration to
// decide whether the delete endpoint exists for this module).
func (h *AccountManagementHandler) Config() ManagedAccountConfig {
	return h.config
}

// managedAuditAction maps a lifecycle verb plus the module's target role to
// the account-management audit action convention used across the generic and
// scoped endpoints (CREATE_ADMIN, UPDATE_ADMIN, ACTIVATE_ADMIN,
// DEACTIVATE_ADMIN, CREATE_PARTNER, ...). The past-tense verbs passed by call
// sites (CREATED / UPDATED / DELETED) are normalised so the recorded action
// reads as the imperative spec convention (Part 8).
// Note: internal audit action identifiers (CREATE_PARTNER, etc.) are kept
// stable for database compatibility; the user-facing label is "Manager".
func managedAuditAction(verb, targetRole string) string {
	switch strings.ToUpper(strings.TrimSpace(verb)) {
	case "CREATED":
		verb = "CREATE"
	case "UPDATED":
		verb = "UPDATE"
	case "DELETED":
		verb = "DELETE"
	case "ACTIVATED":
		verb = "ACTIVATE"
	case "DEACTIVATED":
		verb = "DEACTIVATE"
	}

	return strings.ToUpper(strings.TrimSpace(verb)) + "_" +
		strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(targetRole), " ", "_"))
}

// managedItem serialises a user for list/detail responses. Password material
// is never included; the JSON tags on models.User already omit the hash.
func managedItem(user models.User) gin.H {
	return gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"full_name":     user.FullName,
		"phone":         user.Phone,
		"language":      user.Language,
		"role":          user.Role.Name,
		"status":        user.Status,
		"last_login_at": user.LastLoginAt,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
	}
}

// Page renders the module's management page.
func (h *AccountManagementHandler) Page(c *gin.Context) {
	data := gin.H{
		"title":             h.config.Label + " Management",
		"page_template":     "managed_users_content",
		"managed_label":     h.config.Label,
		"managed_role":      h.config.Role,
		"managed_api":       h.config.APIBase,
		"managed_deletable": h.config.Deletable,
		// The permission that governs account creation for this module
		// (admin.create / partner.create / staff.create / volunteer.create),
		// used for the server-side "+ Add" button visibility check.
		// The user-facing label (e.g. "+ Add Manager") comes from managed_label.
		"managed_create_permission": h.createPermission(),
	}

	// The authenticated actor's ID: the deletion-workflow UI uses it to hide
	// Approve/Reject on the actor's own requests and to hide self-delete
	// controls. UI-only — the backend enforces the four-eyes rule.
	if currentUser, ok := getCurrentUser(c); ok {
		data["managed_actor_id"] = currentUser.ID.String()
	}

	c.HTML(http.StatusOK, "base", PageData(c, data))
}

// createPermission returns the permission governing account creation for
// this module, mirroring requiredTargetPermission + the route gate.
func (h *AccountManagementHandler) createPermission() string {
	if required := requiredTargetPermission(h.config.Role, "create"); required != "" {
		return required
	}
	return strings.ToLower(h.config.Role) + ".create"
}

// List returns the module's accounts with search, status filter and
// server-side pagination. The role filter is forced to the module's target
// role, so a caller can never widen the query to another role.
func (h *AccountManagementHandler) List(c *gin.Context) {
	var query models.UserListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidQueryParameters,
		})
		return
	}

	// Server-side role scoping: the module always lists its own role.
	query.Role = h.config.Role

	users, total, page, pageSize, err := h.userService.ListUsers(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve accounts",
		})
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, managedItem(user))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Accounts retrieved successfully",
		"data":    items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}

// GetByID returns one managed account.
func (h *AccountManagementHandler) GetByID(c *gin.Context) {
	user, err := h.userService.GetUserByID(c.Param("id"))
	if err != nil {
		h.respondUserLookupError(c, err)
		return
	}

	// Defence in depth: an account of another role must never be readable
	// through this module even with a valid ID (e.g. an Admin requesting an
	// Admin account via /staff/:id).
	if user.Role.Name != h.config.Role {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": constants.ErrUserNotFound,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Account retrieved successfully",
		"user":    managedItem(*user),
	})
}

// Create adds a new account with the module's fixed role. The client-supplied
// role field is ignored and overwritten server-side; the service layer then
// re-validates the hierarchy (CanAssignRole) so only a Super Admin or an
// Admin can ever reach the Admin module successfully — every other role is
// rejected regardless of the permissions it holds.
func (h *AccountManagementHandler) Create(c *gin.Context) {
	var request models.CreateManagedAccountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid account information",
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	// Server-side role forcing: a malicious payload that attempts to escalate
	// (e.g. role=Super Admin on POST /admins) is rewritten to the module's
	// role, so the account is always created with the role the endpoint
	// manages. A target role the actor may not assign (e.g. Super Admin from
	// an Admin) is rejected by the hierarchy guard below, and for non-Admin
	// actors the Admin target is rejected outright (canCreateRole).
	request.Role = h.config.Role

	if !canCreateRole(currentUser.Role.Name, h.config.Role) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "This role cannot be assigned by your account.",
		})
		return
	}

	if !hasTargetPermission(c, h.config.Role, "create") {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": services.ErrProtectedPermission.Error(),
		})
		return
	}

	user, err := h.userService.CreateManagedAccount(currentUser.Role.Name, request)
	if err != nil {
		h.respondCreateError(c, err)
		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		managedAuditAction("CREATED", h.config.Role),
		"users",
		user.ID.String(),
		fmt.Sprintf("%s account created (status %s, password setup %s)",
			h.config.Role, user.Status, models.PasswordSetupTemporaryPassword),
	); err != nil {
		// Audit logging failure must not fail the account creation.
	}

	// The temporary-password setup already stored a hashed password and the
	// account is Active, so the user can log in immediately; the plaintext
	// password from the request is never logged or persisted.
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": h.config.Label + " created successfully",
		"user":    managedItem(*user),
	})
}

// Update edits a managed account. Role changes are impossible by design:
// the request model carries no role field and the service rejects role
// mutation, so no actor — including a Super Admin — can escalate through
// this endpoint.
func (h *AccountManagementHandler) Update(c *gin.Context) {
	var request models.UpdateManagedAccountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid account information",
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	if !services.CanManageAccountRole(currentUser.Role.Name, h.config.Role) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": hierarchyMessageForTarget(h.config.Role),
		})
		return
	}

	if !hasTargetPermission(c, h.config.Role, "edit") {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": services.ErrProtectedPermission.Error(),
		})
		return
	}

	user, err := h.userService.UpdateManagedAccount(
		c.Param("id"),
		request,
		currentUser.Role.Name,
	)
	if err != nil {
		h.respondMutationError(c, err, "Unable to update account")
		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		managedAuditAction("UPDATED", h.config.Role),
		"users",
		user.ID.String(),
		fmt.Sprintf("%s account updated", h.config.Role),
	); err != nil {
		// Audit logging failure must not fail the account update.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": h.config.Label + " updated successfully",
		"user":    managedItem(*user),
	})
}

// UpdateStatus activates or deactivates a managed account. Activation
// preserves all historical data (soft status change); deletion is the only
// destructive path and is gated separately. Admin and Manager (Partner role)
// targets carry the dedicated admin.activate / admin.deactivate / partner.* permissions.
func (h *AccountManagementHandler) UpdateStatus(c *gin.Context) {
	var request models.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrStatusRequired,
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	if !services.CanManageAccountRole(currentUser.Role.Name, h.config.Role) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": hierarchyMessageForTarget(h.config.Role),
		})
		return
	}

	// Role-specific permission for the direction of the change
	// (admin.activate vs admin.deactivate, partner.*), no extra permission
	// for staff/volunteer beyond the route's .edit gate.
	action := statusAuditAction(request.Status)
	if !hasTargetPermission(c, h.config.Role, strings.ToLower(action)) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": services.ErrProtectedPermission.Error(),
		})
		return
	}

	if err := h.userService.UpdateUserStatus(
		c.Param("id"),
		request.Status,
		currentUser.Role.Name,
	); err != nil {
		h.respondMutationError(c, err, "Unable to update account status")
		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		managedAuditAction(action, h.config.Role),
		"users",
		c.Param("id"),
		fmt.Sprintf("%s account status changed to %s", h.config.Role, request.Status),
	); err != nil {
		// Audit logging failure must not fail the status update.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Account status updated successfully",
	})
}

// ResetPassword sets a new password for a managed account. The password is
// hashed by the service (never stored or logged in plaintext) and the
// hierarchy is enforced in the service layer.
func (h *AccountManagementHandler) ResetPassword(c *gin.Context) {
	var request models.ResetUserPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "A valid new password is required",
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	if err := h.userService.ResetPassword(
		c.Param("id"),
		request.NewPassword,
		currentUser.Role.Name,
	); err != nil {
		h.respondMutationError(c, err, "Unable to reset account password")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Account password reset successfully",
	})
}

// Delete soft-deletes a managed account (audit history is preserved; the
// login sessions are revoked). Only registered for modules whose role has a
// delete permission in the catalog.
func (h *AccountManagementHandler) Delete(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	if !services.CanManageAccountRole(currentUser.Role.Name, h.config.Role) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": hierarchyMessageForTarget(h.config.Role),
		})
		return
	}

	if !hasTargetPermission(c, h.config.Role, "delete") {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": services.ErrProtectedPermission.Error(),
		})
		return
	}

	targetID := c.Param("id")

	if err := h.userService.DeleteUser(
		targetID,
		currentUser.ID.String(),
		currentUser.Role.Name,
	); err != nil {
		h.respondMutationError(c, err, "Unable to delete account")
		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		managedAuditAction("DELETED", h.config.Role),
		"users",
		targetID,
		fmt.Sprintf("%s account deleted", h.config.Role),
	); err != nil {
		// Audit logging failure must not fail the account deletion.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": h.config.Label + " deleted successfully",
	})
}

// respondUserLookupError maps user-lookup failures to client responses.
func (h *AccountManagementHandler) respondUserLookupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrInvalidUserID):
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidUserID,
		})
	case errors.Is(err, repository.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": constants.ErrUserNotFound,
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve account",
		})
	}
}

// respondCreateError maps managed-account creation failures.
func (h *AccountManagementHandler) respondCreateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrRoleHierarchyViolation):
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "This role cannot be assigned by your account.",
		})
	case errors.Is(err, services.ErrUserAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "That username or email is already in use.",
		})
	case errors.Is(err, services.ErrInvalidRole):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"message": "This role cannot be assigned by your account.",
		})
	case errors.Is(err, services.ErrInvalidPassword):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"message": constants.ErrInvalidPassword,
		})
	case errors.Is(err, services.ErrManagerAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": err.Error(),
		})
	case errors.Is(err, services.ErrInvalidUserStatus),
		errors.Is(err, services.ErrInvalidUserIdentifier):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"message": "Complete the form with a valid email and password setup.",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to create account",
		})
	}
}

// respondMutationError maps managed-account mutation failures.
func (h *AccountManagementHandler) respondMutationError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, services.ErrInvalidUserID):
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidUserID,
		})
	case errors.Is(err, repository.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": constants.ErrUserNotFound,
		})
	case errors.Is(err, services.ErrUserAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "That username or email is already in use.",
		})
	case errors.Is(err, services.ErrInvalidUserStatus):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"message": constants.ErrInvalidUserStatus,
		})
	case errors.Is(err, services.ErrInvalidPassword):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"message": constants.ErrInvalidPassword,
		})
	case errors.Is(err, services.ErrCannotModifySuperAdmin),
		errors.Is(err, services.ErrCannotModifyAdmin),
		errors.Is(err, services.ErrRoleHierarchyViolation),
		errors.Is(err, services.ErrCannotDeleteSelf):
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fallback,
		})
	}
}
