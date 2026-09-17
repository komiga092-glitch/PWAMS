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
	donorService    *services.DonorService
	auditLogService *services.AuditLogService
}

func NewDonorHandler(
	donorService *services.DonorService,
	auditLogServices ...*services.AuditLogService,
) *DonorHandler {
	var auditLogService *services.AuditLogService
	if len(auditLogServices) > 0 {
		auditLogService = auditLogServices[0]
	}

	return &DonorHandler{
		donorService:    donorService,
		auditLogService: auditLogService,
	}
}

func (h *DonorHandler) Create(c *gin.Context) {
	var request models.CreateDonorRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid donor information",
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
			errorResponseMapping{err: services.ErrInvalidPhone, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrDonorAlreadyExists, status: http.StatusConflict, message: err.Error()},
		)
		return
	}

	if currentUser, ok := getCurrentUser(c); ok {
		if err := h.auditLogService.Create(
			currentUser.ID.String(),
			"CREATE",
			"donors",
			donor.ID.String(),
			"Donor registered successfully",
		); err != nil {
			// Audit logging failure must not fail the donor creation.
		}
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
func (h *DonorHandler) Page(c *gin.Context) {
	var query models.DonorListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
			"page_template": "donors_content",
			"title":         "Donors - PWAMS",
			"data":          []gin.H{},
			"search":        "",
			"status":        "",
			"error":         "Invalid query parameters",
		}))
		return
	}

	donors, _, _, _, err :=
		h.donorService.ListDonors(query)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"page_template": "donors_content",
			"title":         "Donors - PWAMS",
			"data":          []gin.H{},
			"search":        query.Search,
			"status":        query.Status,
			"error":         "Unable to retrieve donors",
		}))
		return
	}

	items := make([]gin.H, 0, len(donors))

	for _, donor := range donors {
		items = append(items, gin.H{
			"ID":     donor.ID,
			"Name":   donor.Name,
			"Phone":  donor.Phone,
			"Email":  donor.Email,
			"Type":   donor.DonorType,
			"Status": donor.Status,
		})
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "donors_content",
		"title":         "Donors - PWAMS",
		"data":          items,
		"search":        query.Search,
		"status":        query.Status,
	}))
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
		if errors.Is(err, services.ErrInvalidDonorType) ||
			errors.Is(err, services.ErrInvalidDonorStatus) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
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
			errorResponseMapping{err: services.ErrInvalidPhone, status: http.StatusUnprocessableEntity, message: err.Error()},
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

func (h *DonorHandler) ViewPage(c *gin.Context) {
	donorID := c.Param("id")

	donor, err := h.donorService.GetDonorByID(donorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorID):
			c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
				"page_template": "donors_content",
				"title":         "Donors - PWAMS",
				"error":         constants.ErrInvalidDonorID,
			}))

		case errors.Is(err, repository.ErrDonorNotFound):
			c.HTML(http.StatusNotFound, "base", PageData(c, gin.H{
				"page_template": "donors_content",
				"title":         "Donors - PWAMS",
				"error":         constants.ErrDonorNotFound,
			}))

		default:
			c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
				"page_template": "donors_content",
				"title":         "Donors - PWAMS",
				"error":         constants.ErrUnableToRetrieveDonor,
			}))
		}

		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "donor_view_content",
		"title":         "View Donor - PWAMS",
		"donor":         donor,
	}))
}

func (h *DonorHandler) EditPage(c *gin.Context) {
	donorID := c.Param("id")

	donor, err := h.donorService.GetDonorByID(donorID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidDonorID):
			c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
				"page_template": "donors_content",
				"title":         "Donors - PWAMS",
				"error":         constants.ErrInvalidDonorID,
			}))

		case errors.Is(err, repository.ErrDonorNotFound):
			c.HTML(http.StatusNotFound, "base", PageData(c, gin.H{
				"page_template": "donors_content",
				"title":         "Donors - PWAMS",
				"error":         constants.ErrDonorNotFound,
			}))

		default:
			c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
				"page_template": "donors_content",
				"title":         "Donors - PWAMS",
				"error":         constants.ErrUnableToRetrieveDonor,
			}))
		}

		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "donor_edit_content",
		"title":         "Edit Donor - PWAMS",
		"donor":         donor,
	}))
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
