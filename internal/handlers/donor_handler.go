package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type DonorHandler struct {
	donorService *services.DonorService
}

func NewDonorHandler(
	donorService *services.DonorService,
) *DonorHandler {
	return &DonorHandler{
		donorService: donorService,
	}
}

func (h *DonorHandler) Create(c *gin.Context) {
	var request models.CreateDonorRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid donor information",
			"error":   err.Error(),
		})
		return
	}

	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
		})
		return
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid authentication context",
		})
		return
	}

	donor, err := h.donorService.CreateDonor(
		request,
		currentUser.ID,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorType):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrIndividualDonorIdentityRequired):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrOrganizationDetailsRequired):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrDonorAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create donor",
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Donor registered successfully",
		"donor": gin.H{
			"id":                      donor.ID,
			"name":                    donor.Name,
			"donor_type":              donor.DonorType,
			"nic_passport":            donor.NICPassport,
			"organization_name":       donor.OrganizationName,
			"registration_number":     donor.RegistrationNumber,
			"phone":                   donor.Phone,
			"email":                   donor.Email,
			"address":                 donor.Address,
			"contact_person_name":     donor.ContactPersonName,
			"contact_person_phone":    donor.ContactPersonPhone,
			"preferred_donation_type": donor.PreferredDonationType,
			"notes":                   donor.Notes,
			"status":                  donor.Status,
			"created_by_id":           donor.CreatedByID,
		},
	})
}
