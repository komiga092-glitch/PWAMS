package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type DonationHandler struct {
	donationService *services.DonationService
}

func NewDonationHandler(
	donationService *services.DonationService,
) *DonationHandler {
	return &DonationHandler{
		donationService: donationService,
	}
}

func (h *DonationHandler) Create(c *gin.Context) {
	var request models.CreateDonationRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid donation information",
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

	donation, err := h.donationService.CreateDonation(
		request,
		currentUser.ID,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid donor ID",
			})

		case errors.Is(err, repository.ErrDonorNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Donor not found",
			})

		case errors.Is(err, services.ErrDonorNotActive):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

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

		case errors.Is(err, services.ErrInvalidDonationType),
			errors.Is(err, services.ErrCashAmountRequired),
			errors.Is(err, services.ErrItemDetailsRequired),
			errors.Is(err, services.ErrInvalidDonationDate):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrDonationReferenceExists):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create donation",
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Donation registered successfully",
		"donation": gin.H{
			"id":            donation.ID,
			"donor_id":      donation.DonorID,
			"person_id":     donation.PersonID,
			"donation_type": donation.DonationType,
			"amount":        donation.Amount,
			"currency":      donation.Currency,
			"item_name":     donation.ItemName,
			"quantity":      donation.Quantity,
			"unit":          donation.Unit,
			"description":   donation.Description,
			"donation_date": donation.DonationDate,
			"reference_no":  donation.ReferenceNo,
			"status":        donation.Status,
			"created_by_id": donation.CreatedByID,
		},
	})
}
