package handlers

import (
	"log"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type DonationHandler struct {
	donationService *services.DonationService
	donorService    *services.DonorService
}

func NewDonationHandler(
	donationService *services.DonationService,
	donorService *services.DonorService,
) *DonationHandler {
	return &DonationHandler{
		donationService: donationService,
		donorService:    donorService,
	}
}

func (h *DonationHandler) Page(c *gin.Context) {
	donors, _, _, _, err := h.donorService.ListDonors(models.DonorListQuery{
		Status:   models.DonorStatusActive,
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"page_template": "donations_content",
			"title":         "Donations",
			"data":          []gin.H{},
			"donors":        []models.Donor{},
			"error":         "Unable to retrieve donors",
		}))
		return
	}
	sort.SliceStable(donors, func(i, j int) bool {
		return donors[i].Name < donors[j].Name
	})

	var query models.DonationListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{"page_template": "donations_content", "title": "Donations", "data": []gin.H{}, "donors": donors, "error": "Invalid query parameters"}))
		return
	}
	donationRecords, _, _, _, err := h.donationService.ListDonations(query)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{"page_template": "donations_content", "title": "Donations", "data": []gin.H{}, "donors": donors, "error": "Unable to retrieve donations"}))
		return
	}
	items := make([]gin.H, 0, len(donationRecords))
	for _, donation := range donationRecords {
		donorName := "Unknown donor"
		if donation.Donor.ID != uuid.Nil {
			donorName = donation.Donor.Name
		}
		items = append(items, gin.H{"ID": donation.ID, "DonorID": donation.DonorID, "DonorName": donorName, "Amount": donation.Amount, "DonationDate": donation.DonationDate, "Description": donation.Description, "Status": donation.Status})
	}
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{"page_template": "donations_content", "title": "Donations", "data": items, "donors": donors}))
}

func (h *DonationHandler) Create(c *gin.Context) {
	var request models.CreateDonationRequest

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid donation information",
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
