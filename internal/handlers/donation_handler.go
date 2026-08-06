package handlers

import (
	"errors"
	"log"
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

func (h *DonationHandler) List(c *gin.Context) {
	var query models.DonationListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid query parameters",
		})
		return
	}

	donations, total, page, pageSize, err :=
		h.donationService.ListDonations(query)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid donor ID",
			})

		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid person ID",
			})

		case errors.Is(err, services.ErrInvalidDonationType):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrInvalidDonationStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrInvalidDonationDateRange):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve donations",
			})
		}

		return
	}

	items := make([]gin.H, 0, len(donations))

	for _, donation := range donations {
		var personData any

		if donation.Person != nil {
			personData = gin.H{
				"id":           donation.Person.ID,
				"full_name":    donation.Person.FullName,
				"nic_passport": donation.Person.NICPassport,
			}
		}

		items = append(items, gin.H{
			"id":           donation.ID,
			"reference_no": donation.ReferenceNo,
			"donor": gin.H{
				"id":         donation.Donor.ID,
				"name":       donation.Donor.Name,
				"donor_type": donation.Donor.DonorType,
			},
			"person":        personData,
			"donation_type": donation.DonationType,
			"amount":        donation.Amount,
			"currency":      donation.Currency,
			"item_name":     donation.ItemName,
			"quantity":      donation.Quantity,
			"unit":          donation.Unit,
			"description":   donation.Description,
			"donation_date": donation.DonationDate,
			"status":        donation.Status,
			"created_by":    donation.CreatedBy.Username,
			"created_at":    donation.CreatedAt,
			"updated_at":    donation.UpdatedAt,
		})
	}

	totalPages := 0

	if total > 0 {
		totalPages = int(
			(total + int64(pageSize) - 1) /
				int64(pageSize),
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donations retrieved successfully",
		"data":    items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}

func (h *DonationHandler) GetByID(c *gin.Context) {
	donationID := c.Param("id")

	donation, err := h.donationService.GetDonationByID(
		donationID,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonationID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid donation ID",
			})

		case errors.Is(err, repository.ErrDonationNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Donation not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve donation",
			})
		}

		return
	}

	var personData any

	if donation.Person != nil {
		personData = gin.H{
			"id":           donation.Person.ID,
			"full_name":    donation.Person.FullName,
			"nic_passport": donation.Person.NICPassport,
			"phone":        donation.Person.Phone,
			"status":       donation.Person.Status,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donation retrieved successfully",
		"donation": gin.H{
			"id":           donation.ID,
			"reference_no": donation.ReferenceNo,

			"donor": gin.H{
				"id":         donation.Donor.ID,
				"name":       donation.Donor.Name,
				"donor_type": donation.Donor.DonorType,
				"phone":      donation.Donor.Phone,
				"email":      donation.Donor.Email,
				"status":     donation.Donor.Status,
			},

			"person":        personData,
			"donation_type": donation.DonationType,
			"amount":        donation.Amount,
			"currency":      donation.Currency,
			"item_name":     donation.ItemName,
			"quantity":      donation.Quantity,
			"unit":          donation.Unit,
			"description":   donation.Description,
			"donation_date": donation.DonationDate,
			"status":        donation.Status,

			"created_by": gin.H{
				"id":       donation.CreatedBy.ID,
				"username": donation.CreatedBy.Username,
				"email":    donation.CreatedBy.Email,
			},

			"created_at": donation.CreatedAt,
			"updated_at": donation.UpdatedAt,
		},
	})
}

func (h *DonationHandler) Update(c *gin.Context) {
	donationID := c.Param("id")

	var request models.UpdateDonationRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid donation information",
			"error":   err.Error(),
		})
		return
	}

	donation, err := h.donationService.UpdateDonation(
		donationID,
		request,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonationID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid donation ID",
			})

		case errors.Is(err, repository.ErrDonationNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Donation not found",
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
			errors.Is(err, services.ErrInvalidDonationStatus),
			errors.Is(err, services.ErrCashAmountRequired),
			errors.Is(err, services.ErrItemDetailsRequired),
			errors.Is(err, services.ErrInvalidDonationDate):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update donation",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donation updated successfully",
		"donation": gin.H{
			"id":            donation.ID,
			"reference_no":  donation.ReferenceNo,
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
			"status":        donation.Status,
			"updated_at":    donation.UpdatedAt,
		},
	})
}
func (h *DonationHandler) UpdateStatus(c *gin.Context) {
	donationID := c.Param("id")

	var request models.UpdateDonationStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Status is required",
		})
		return
	}

	err := h.donationService.UpdateDonationStatus(
		donationID,
		request.Status,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonationID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid donation ID",
			})

		case errors.Is(err, services.ErrInvalidDonationStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, repository.ErrDonationNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Donation not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update donation status",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donation status updated successfully",
	})
}
func (h *DonationHandler) Delete(c *gin.Context) {
	donationID := c.Param("id")

	err := h.donationService.DeleteDonation(donationID)
	if err != nil {
		log.Printf("delete donation error: %T - %v", err, err)

		switch {
		case errors.Is(err, services.ErrInvalidDonationID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid donation ID",
			})

		case errors.Is(err, repository.ErrDonationNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Donation not found",
			})

		case errors.Is(err, services.ErrConfirmedDonationCannotDelete):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete donation",
				"error":   err.Error(),
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donation deleted successfully",
	})
}
