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

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	donor, err := h.donorService.CreateDonor(
		request,
		currentUser.ID,
	)

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to create donor",
			errorResponseMapping{err: services.ErrInvalidDonorType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrIndividualDonorIdentityRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrOrganizationDetailsRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrDonorAlreadyExists, status: http.StatusConflict, message: err.Error()},
		)
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

func (h *DonorHandler) List(c *gin.Context) {
	var query models.DonorListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid query parameters",
		})
		return
	}

	donors, total, page, pageSize, err :=
		h.donorService.ListDonors(query)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve donors",
		})
		return
	}

	items := make([]gin.H, 0, len(donors))

	for _, donor := range donors {
		items = append(items, gin.H{
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
			"created_by":              donor.CreatedBy.Username,
			"created_at":              donor.CreatedAt,
			"updated_at":              donor.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Donors retrieved successfully",
		"data":       items,
		"pagination": buildPagination(total, page, pageSize),
	})
}

func (h *DonorHandler) GetByID(c *gin.Context) {
	donorID := c.Param("id")

	donor, err := h.donorService.GetDonorByID(donorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidDonorID,
			})

		case errors.Is(err, repository.ErrDonorNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrDonorNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToRetrieveDonor,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donor retrieved successfully",
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
			"created_by":              donor.CreatedBy.Username,
			"created_at":              donor.CreatedAt,
			"updated_at":              donor.UpdatedAt,
		},
	})
}

func (h *DonorHandler) Update(c *gin.Context) {
	donorID := c.Param("id")

	var request models.UpdateDonorRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid donor information",
			"error":   err.Error(),
		})
		return
	}

	donor, err := h.donorService.UpdateDonor(
		donorID,
		request,
	)
	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to update donor",
			errorResponseMapping{err: services.ErrInvalidDonorID, status: http.StatusBadRequest, message: constants.ErrInvalidDonorID},
			errorResponseMapping{err: repository.ErrDonorNotFound, status: http.StatusNotFound, message: constants.ErrDonorNotFound},
			errorResponseMapping{err: services.ErrInvalidDonorType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrIndividualDonorIdentityRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrOrganizationDetailsRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidDonorStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrDonorAlreadyExists, status: http.StatusConflict, message: err.Error()},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donor updated successfully",
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
			"updated_at":              donor.UpdatedAt,
		},
	})
}

func (h *DonorHandler) UpdateStatus(c *gin.Context) {
	donorID := c.Param("id")

	var request models.UpdateDonorStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Status is required",
		})
		return
	}

	err := h.donorService.UpdateDonorStatus(
		donorID,
		request.Status,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidDonorID,
			})

		case errors.Is(err, services.ErrInvalidDonorStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, repository.ErrDonorNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrDonorNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update donor status",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donor status updated successfully",
	})
}

func (h *DonorHandler) Delete(c *gin.Context) {
	donorID := c.Param("id")

	err := h.donorService.DeleteDonor(donorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidDonorID,
			})

		case errors.Is(err, repository.ErrDonorNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrDonorNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToRetrieveDonor,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donor deleted successfully",
	})
}
