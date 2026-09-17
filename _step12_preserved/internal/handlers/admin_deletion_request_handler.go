package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// AdminDeletionRequestHandler exposes the supervised Admin account deletion
// workflow (POST create / GET list / POST approve / POST reject). The
// permission gates (admin.delete.request / admin.delete.approve /
// admin.delete.reject) are enforced by the route middleware; the service
// layer additionally enforces the role hierarchy and the four-eyes rule
// (approver != requester) independently of these gates.
type AdminDeletionRequestHandler struct {
	deletionService *services.AdminDeletionRequestService
}

func NewAdminDeletionRequestHandler(
	deletionService *services.AdminDeletionRequestService,
) *AdminDeletionRequestHandler {
	return &AdminDeletionRequestHandler{deletionService: deletionService}
}

// Create records a request to delete an Admin account.
func (h *AdminDeletionRequestHandler) Create(c *gin.Context) {
	var request models.CreateAdminDeletionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "A valid target user is required",
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	record, err := h.deletionService.Create(currentUser, request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeletionSelfRequest):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionTargetRole),
			errors.Is(err, services.ErrDeletionHierarchy):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionDuplicate):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionTargetNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to create the deletion request"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Admin deletion request created",
		"request": record,
	})
}

// List returns pending Admin deletion requests, newest first.
func (h *AdminDeletionRequestHandler) List(c *gin.Context) {
	var query models.AdminDeletionRequestListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid query parameters"})
		return
	}

	records, total, err := h.deletionService.ListPending(query.Page, query.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve deletion requests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"requests": records,
		"total":    total,
	})
}

// Approve resolves a pending request and deletes the target Admin account.
// The service layer enforces approver != requester (four-eyes rule) and
// approver != target.
func (h *AdminDeletionRequestHandler) Approve(c *gin.Context) {
	h.respond(c, true)
}

// Reject resolves a pending request; the target account remains untouched.
// The service layer enforces responder != requester (four-eyes rule) and
// responder != target.
func (h *AdminDeletionRequestHandler) Reject(c *gin.Context) {
	h.respond(c, false)
}

// Cancel withdraws a pending request on behalf of its requester. Only the
// requester may cancel and only while the request is still pending.
func (h *AdminDeletionRequestHandler) Cancel(c *gin.Context) {
	requestID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid deletion request ID"})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	record, err := h.deletionService.Cancel(currentUser, requestID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeletionRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionAlreadyResolved):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionCancelForbidden),
			errors.Is(err, services.ErrDeletionHierarchy):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to cancel the deletion request"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Admin deletion request cancelled",
		"request": record,
	})
}

func (h *AdminDeletionRequestHandler) respond(c *gin.Context, approve bool) {
	requestID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid deletion request ID"})
		return
	}

	var request models.RespondAdminDeletionRequest
	_ = c.ShouldBindJSON(&request)

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	var record *models.AdminDeletionRequest
	if approve {
		record, err = h.deletionService.Approve(currentUser, requestID, request.Response)
	} else {
		record, err = h.deletionService.Reject(currentUser, requestID, request.Response)
	}
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeletionRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionAlreadyResolved):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionSelfApproval),
			errors.Is(err, services.ErrDeletionTargetApproval),
			errors.Is(err, services.ErrDeletionTargetRole),
			errors.Is(err, services.ErrDeletionHierarchy),
			errors.Is(err, services.ErrCannotDeleteSelf),
			errors.Is(err, services.ErrCannotModifySuperAdmin),
			errors.Is(err, services.ErrCannotModifyAdmin),
			errors.Is(err, services.ErrRoleHierarchyViolation):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeletionTargetNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to respond to the deletion request"})
		}
		return
	}

	message := "Admin deletion request rejected"
	if approve {
		message = "Admin deletion request approved; account deleted"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"request": record,
	})
}
