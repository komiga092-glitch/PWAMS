package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type AidRequestHandler struct {
	aidRequestService *services.AidRequestService
}

func NewAidRequestHandler(
	aidRequestService *services.AidRequestService,
) *AidRequestHandler {
	return &AidRequestHandler{
		aidRequestService: aidRequestService,
	}
}

func (h *AidRequestHandler) Create(c *gin.Context) {
	var request models.CreateAidRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid aid request information",
			"error":   err.Error(),
		})
		return
	}

	value, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
		})
		return
	}

	currentUser, ok := value.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid authentication context",
		})
		return
	}

	aidRequest, err := h.aidRequestService.CreateAidRequest(
		request,
		currentUser.ID,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid person ID",
			})

		case errors.Is(err, repository.ErrPersonNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Person not found",
			})

		case errors.Is(err, services.ErrInvalidAidType),
			errors.Is(err, services.ErrInvalidAidPriority),
			errors.Is(err, services.ErrInvalidAidRequestDate),
			errors.Is(err, services.ErrInvalidNeededByDate):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create aid request",
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Aid request created successfully",
		"aid_request": gin.H{
			"id":               aidRequest.ID,
			"person_id":        aidRequest.PersonID,
			"aid_type":         aidRequest.AidType,
			"priority":         aidRequest.Priority,
			"title":            aidRequest.Title,
			"description":      aidRequest.Description,
			"requested_amount": aidRequest.RequestedAmount,
			"approved_amount":  aidRequest.ApprovedAmount,
			"currency":         aidRequest.Currency,
			"request_date":     aidRequest.RequestDate,
			"needed_by":        aidRequest.NeededBy,
			"status":           aidRequest.Status,
			"created_by_id":    aidRequest.CreatedByID,
		},
	})
}
