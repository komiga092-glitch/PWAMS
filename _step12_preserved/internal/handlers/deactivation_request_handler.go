package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type DeactivationRequestHandler struct {
	deactivationService *services.DeactivationRequestService
}

func NewDeactivationRequestHandler(
	deactivationService *services.DeactivationRequestService,
) *DeactivationRequestHandler {
	return &DeactivationRequestHandler{deactivationService: deactivationService}
}

// Create records a request to deactivate another Super Admin.
func (h *DeactivationRequestHandler) Create(c *gin.Context) {
	var request models.CreateDeactivationRequest
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

	record, err := h.deactivationService.Create(currentUser.ID, request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeactivationSelfRequest):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeactivationTargetRole):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeactivationDuplicate):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeactivationTargetNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to create the deactivation request"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Deactivation request created",
		"request": record,
	})
}

// List returns deactivation requests involving the current user.
func (h *DeactivationRequestHandler) List(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	var query models.DeactivationRequestListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid query parameters"})
		return
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	records, total, err := h.deactivationService.ListForUser(currentUser.ID, query.Page, query.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve deactivation requests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"requests": records,
		"total":    total,
	})
}

// Accept resolves a pending request and deactivates the target (the responder).
func (h *DeactivationRequestHandler) Accept(c *gin.Context) {
	h.respond(c, true)
}

// Reject resolves a pending request; the target remains active.
func (h *DeactivationRequestHandler) Reject(c *gin.Context) {
	h.respond(c, false)
}

func (h *DeactivationRequestHandler) respond(c *gin.Context, accept bool) {
	requestID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid deactivation request ID"})
		return
	}

	var request models.RespondDeactivationRequest
	_ = c.ShouldBindJSON(&request)

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	record, err := h.deactivationService.Respond(requestID, currentUser.ID, accept, request.Response)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeactivationRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeactivationAlreadyResolved):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, services.ErrDeactivationNotResponder):
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to respond to the deactivation request"})
		}
		return
	}

	message := "Deactivation request rejected"
	if accept {
		message = "Deactivation request accepted; account deactivated"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"request": record,
	})
}
