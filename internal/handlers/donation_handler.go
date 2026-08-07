package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
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

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	donation, err := h.donationService.CreateDonation(
		request,
		currentUser.ID,
	)

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to create donation",
			errorResponseMapping{err: services.ErrInvalidDonorID, status: http.StatusBadRequest, message: "Invalid donor ID"},
			errorResponseMapping{err: repository.ErrDonorNotFound, status: http.StatusNotFound, message: "Donor not found"},
			errorResponseMapping{err: services.ErrDonorNotActive, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: constants.ErrInvalidPersonID},
			errorResponseMapping{err: repository.ErrPersonNotFound, status: http.StatusNotFound, message: "Person not found"},
			errorResponseMapping{err: services.ErrInvalidDonationType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrCashAmountRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrItemDetailsRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidDonationDate, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrDonationReferenceExists, status: http.StatusConflict, message: err.Error()},
		)
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
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to retrieve donations",
			errorResponseMapping{err: services.ErrInvalidDonorID, status: http.StatusBadRequest, message: "Invalid donor ID"},
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: constants.ErrInvalidPersonID},
			errorResponseMapping{err: services.ErrInvalidDonationType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidDonationStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidDonationDateRange, status: http.StatusUnprocessableEntity, message: err.Error()},
		)
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

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    constants.ErrDonationsRetrievedSuccessfully,
		"data":       items,
		"pagination": buildPagination(total, page, pageSize),
	})
}

func (h *DonationHandler) GetByID(c *gin.Context) {
	donationID := c.Param("id")

	donation, err := h.donationService.GetDonationByID(
		donationID,
	)

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToRetrieveDonation,
			errorResponseMapping{err: services.ErrInvalidDonationID, status: http.StatusBadRequest, message: constants.ErrInvalidDonationID},
			errorResponseMapping{err: repository.ErrDonationNotFound, status: http.StatusNotFound, message: constants.ErrDonationNotFound},
		)
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
		"message": constants.ErrDonationRetrievedSuccessfully,
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
			"message": constants.ErrInvalidDonationInfo,
			"error":   err.Error(),
		})
		return
	}

	donation, err := h.donationService.UpdateDonation(
		donationID,
		request,
	)

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToUpdateDonation,
			errorResponseMapping{err: services.ErrInvalidDonationID, status: http.StatusBadRequest, message: constants.ErrInvalidDonationID},
			errorResponseMapping{err: repository.ErrDonationNotFound, status: http.StatusNotFound, message: constants.ErrDonationNotFound},
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: constants.ErrInvalidPersonID},
			errorResponseMapping{err: repository.ErrPersonNotFound, status: http.StatusNotFound, message: constants.ErrPersonNotFound},
			errorResponseMapping{err: services.ErrInvalidDonationType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidDonationStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrCashAmountRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrItemDetailsRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidDonationDate, status: http.StatusUnprocessableEntity, message: err.Error()},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrDonationUpdatedSuccessfully,
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
			"message": constants.ErrDonationStatusRequired,
		})
		return
	}

	err := h.donationService.UpdateDonationStatus(
		donationID,
		request.Status,
	)

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToUpdateDonationStatus,
			errorResponseMapping{err: services.ErrInvalidDonationID, status: http.StatusBadRequest, message: constants.ErrInvalidDonationID},
			errorResponseMapping{err: services.ErrInvalidDonationStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: repository.ErrDonationNotFound, status: http.StatusNotFound, message: constants.ErrDonationNotFound},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrDonationStatusUpdatedSuccessfully,
	})
}
func (h *DonationHandler) Delete(c *gin.Context) {
	donationID := c.Param("id")

	err := h.donationService.DeleteDonation(donationID)
	if err != nil {
		log.Printf("delete donation error: %T - %v", err, err)

		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToDeleteDonation,
			errorResponseMapping{err: services.ErrInvalidDonationID, status: http.StatusBadRequest, message: constants.ErrInvalidDonationID},
			errorResponseMapping{err: repository.ErrDonationNotFound, status: http.StatusNotFound, message: constants.ErrDonationNotFound},
			errorResponseMapping{err: services.ErrConfirmedDonationCannotDelete, status: http.StatusConflict, message: err.Error()},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrDonationDeletedSuccessfully,
	})
}
