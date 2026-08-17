package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type CareProvidedHandler struct {
	careProvidedService *services.CareProvidedService
}

func NewCareProvidedHandler(
	careProvidedService *services.CareProvidedService,
) *CareProvidedHandler {
	return &CareProvidedHandler{
		careProvidedService: careProvidedService,
	}
}

func (h *CareProvidedHandler) Create(c *gin.Context) {
	var request models.CreateCareProvidedRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid care provided information",
			"error":   err.Error(),
		})
		return
	}

	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return
	}

	record, err := h.careProvidedService.CreateCareProvided(
		request,
		currentUser.ID.String(),
	)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Care provided record created successfully",
		"data":    record,
	})
}

func (h *CareProvidedHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	records, total, currentPage, totalPages, err :=
		h.careProvidedService.ListCareProvided(
			page,
			pageSize,
		)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve care provided records",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Care provided records retrieved successfully",
		"data":    records,
		"pagination": gin.H{
			"page":        currentPage,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}

func (h *CareProvidedHandler) GetByID(c *gin.Context) {
	record, err := h.careProvidedService.GetCareProvidedByID(
		c.Param("id"),
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCareProvidedID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid care provided ID",
			})

		case errors.Is(err, repository.ErrCareProvidedNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Care provided record not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve care provided record",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Care provided record retrieved successfully",
		"data":    record,
	})
}

func (h *CareProvidedHandler) Update(c *gin.Context) {
	var request models.UpdateCareProvidedRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid care provided information",
			"error":   err.Error(),
		})
		return
	}

	record, err := h.careProvidedService.UpdateCareProvided(
		c.Param("id"),
		request,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCareProvidedID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid care provided ID",
			})

		case errors.Is(err, repository.ErrCareProvidedNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Care provided record not found",
			})

		case errors.Is(err, services.ErrCareAlreadyCompleted):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Care provided record updated successfully",
		"data":    record,
	})
}

func (h *CareProvidedHandler) UpdateStatus(c *gin.Context) {
	var request models.UpdateCareProvidedStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Status is required",
		})
		return
	}

	err := h.careProvidedService.UpdateCareProvidedStatus(
		c.Param("id"),
		request.Status,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCareProvidedID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid care provided ID",
			})

		case errors.Is(err, services.ErrInvalidCareProvidedStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": "Invalid care provided status",
			})

		case errors.Is(err, repository.ErrCareProvidedNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Care provided record not found",
			})

		case errors.Is(err, services.ErrCareAlreadyCompleted):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update care provided status",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Care provided status updated successfully",
	})
}

func (h *CareProvidedHandler) Delete(c *gin.Context) {
	err := h.careProvidedService.DeleteCareProvided(
		c.Param("id"),
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCareProvidedID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid care provided ID",
			})

		case errors.Is(err, repository.ErrCareProvidedNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Care provided record not found",
			})

		case errors.Is(err, services.ErrCareAlreadyCompleted):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete care provided record",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Care provided record deleted successfully",
	})
}
