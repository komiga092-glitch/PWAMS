package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// AdminDeletionHandler implements the admin-deletion approval
// workflow: deleting an Admin requires a request plus approval by
// another authorized Admin (or Super Admin). The requester cannot
// approve their own request and the target cannot decide it.
type AdminDeletionHandler struct {
	userService     *services.UserService
	auditLogService *services.AuditLogService
}

func NewAdminDeletionHandler(
	userService *services.UserService,
	auditLogService *services.AuditLogService,
) *AdminDeletionHandler {
	return &AdminDeletionHandler{
		userService:     userService,
		auditLogService: auditLogService,
	}
}

type createAdminDeletionRequest struct {
	TargetUserID string `json:"target_user_id" binding:"required"`
}

// Request records a Pending admin deletion request.
//
// @Summary Request admin deletion
// @Description Request approval to delete an Admin account (requires approval by another authorized Admin).
// @Tags Users
// @Accept json
// @Produce json
// @Param request body createAdminDeletionRequest true "Target user"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Router /admin-deletion-requests [post]
func (h *AdminDeletionHandler) Request(c *gin.Context) {
	currentUser, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	var body createAdminDeletionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Target user ID is required",
		})
		return
	}

	request, err := h.userService.RequestAdminDeletion(
		currentUser.ID.String(),
		body.TargetUserID,
	)
	if err != nil {
		h.respondError(c, err, "Unable to create admin deletion request")
		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"ADMIN_DELETION_REQUESTED",
		"users",
		body.TargetUserID,
		"Admin deletion approval requested",
	); err != nil {
		// Audit logging failure must not fail the request creation.
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Admin deletion request created; approval by another authorized Admin is required",
		"request": gin.H{
			"id":              request.ID,
			"target_user_id":  request.TargetUserID,
			"requested_by_id": request.RequestedByID,
			"status":          request.Status,
		},
	})
}

// Approve approves a Pending request and deletes the target Admin.
func (h *AdminDeletionHandler) Approve(c *gin.Context) {
	h.decide(c, true)
}

// Reject rejects a Pending request.
func (h *AdminDeletionHandler) Reject(c *gin.Context) {
	h.decide(c, false)
}

func (h *AdminDeletionHandler) decide(c *gin.Context, approve bool) {
	currentUser, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	requestID := c.Param("requestId")

	var err error
	if approve {
		err = h.userService.ApproveAdminDeletion(
			currentUser.ID.String(),
			requestID,
		)
	} else {
		err = h.userService.RejectAdminDeletion(
			currentUser.ID.String(),
			requestID,
		)
	}

	if err != nil {
		h.respondError(
			c,
			err,
			"Unable to process admin deletion request",
		)
		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"ADMIN_DELETION_DECIDED",
		"users",
		requestID,
		map[bool]string{true: "approved", false: "rejected"}[approve],
	); err != nil {
		// Audit logging failure must not fail the decision.
	}

	message := "Admin deletion request rejected"
	if approve {
		message = "Admin deletion request approved and target deleted"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
	})
}

func (h *AdminDeletionHandler) respondError(
	c *gin.Context,
	err error,
	fallback string,
) {
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

	case errors.Is(err, repository.ErrAdminDeletionRequestNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Admin deletion request not found",
		})

	case errors.Is(err, services.ErrCannotDeleteSelf):
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": constants.ErrCannotDeleteSelf,
		})

	case errors.Is(err, services.ErrAdminDeletionNotAuthorized),
		errors.Is(err, services.ErrCannotModifySuperAdmin),
		errors.Is(err, services.ErrAdminDeletionApprovalRequired):
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})

	case errors.Is(err, services.ErrAdminDeletionRequestInvalid):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "message": err.Error()})

	case errors.Is(err, services.ErrLastActiveAdmin),
		errors.Is(err, services.ErrLastActiveSuperAdmin):
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fallback,
		})
	}
}

// getCurrentUser returns the authenticated user from the Gin context.
func (h *AdminDeletionHandler) getCurrentUser(
	c *gin.Context,
) (*models.User, bool) {
	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return nil, false
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return nil, false
	}

	return currentUser, true
}
