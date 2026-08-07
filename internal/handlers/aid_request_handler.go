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
			"message": constants.ErrInvalidAidRequestInfo,
			"error":  err.Error(),
		})
		return
	}

	value, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return
	}

	currentUser, ok := value.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
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
func (h *AidRequestHandler) List(c *gin.Context) {
	var query models.AidRequestListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid query parameters",
		})
		return
	}

	aidRequests, total, page, pageSize, err :=
		h.aidRequestService.ListAidRequests(query)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})

		case errors.Is(err, services.ErrInvalidAidType),
			errors.Is(err, services.ErrInvalidAidPriority),
			errors.Is(err, services.ErrInvalidAidStatus),
			errors.Is(err, services.ErrInvalidAidDateRange):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve aid requests",
			})
		}

		return
	}

	items := make([]gin.H, 0, len(aidRequests))

	for _, aidRequest := range aidRequests {
		var reviewedBy any

		if aidRequest.ReviewedBy != nil {
			reviewedBy = gin.H{
				"id":       aidRequest.ReviewedBy.ID,
				"username": aidRequest.ReviewedBy.Username,
			}
		}

		items = append(items, gin.H{
			"id": aidRequest.ID,

			"person": gin.H{
				"id":           aidRequest.Person.ID,
				"full_name":    aidRequest.Person.FullName,
				"nic_passport": aidRequest.Person.NICPassport,
			},

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
			"review_notes":     aidRequest.ReviewNotes,
			"reviewed_by":      reviewedBy,
			"reviewed_at":      aidRequest.ReviewedAt,
			"created_by":       aidRequest.CreatedBy.Username,
			"created_at":       aidRequest.CreatedAt,
			"updated_at":       aidRequest.UpdatedAt,
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
		"message": "Aid requests retrieved successfully",
		"data":    items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}
func (h *AidRequestHandler) GetByID(c *gin.Context) {
	aidRequestID := c.Param("id")

	aidRequest, err :=
		h.aidRequestService.GetAidRequestByID(aidRequestID)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAidRequestID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidAidRequestID,
			})

		case errors.Is(err, repository.ErrAidRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Aid request not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve aid request",
			})
		}

		return
	}

	var reviewedBy any

	if aidRequest.ReviewedBy != nil {
		reviewedBy = gin.H{
			"id":       aidRequest.ReviewedBy.ID,
			"username": aidRequest.ReviewedBy.Username,
			"email":    aidRequest.ReviewedBy.Email,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Aid request retrieved successfully",
		"aid_request": gin.H{
			"id": aidRequest.ID,

			"person": gin.H{
				"id":           aidRequest.Person.ID,
				"full_name":    aidRequest.Person.FullName,
				"nic_passport": aidRequest.Person.NICPassport,
				"phone":        aidRequest.Person.Phone,
				"email":        aidRequest.Person.Email,
				"address":      aidRequest.Person.Address,
				"status":       aidRequest.Person.Status,
			},

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
			"review_notes":     aidRequest.ReviewNotes,
			"reviewed_by":      reviewedBy,
			"reviewed_at":      aidRequest.ReviewedAt,

			"created_by": gin.H{
				"id":       aidRequest.CreatedBy.ID,
				"username": aidRequest.CreatedBy.Username,
				"email":    aidRequest.CreatedBy.Email,
			},

			"created_at": aidRequest.CreatedAt,
			"updated_at": aidRequest.UpdatedAt,
		},
	})
}
func (h *AidRequestHandler) Update(c *gin.Context) {
	aidRequestID := c.Param("id")

	var request models.UpdateAidRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidAidRequestInfo,
			"error":   err.Error(),
		})
		return
	}

	aidRequest, err :=
		h.aidRequestService.UpdateAidRequest(
			aidRequestID,
			request,
		)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAidRequestID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidAidRequestID,
			})

		case errors.Is(err, repository.ErrAidRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrAidRequestNotFound,
			})

		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})

		case errors.Is(err, repository.ErrPersonNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrPersonNotFound,
			})

		case errors.Is(err, services.ErrInvalidAidType),
			errors.Is(err, services.ErrInvalidAidPriority),
			errors.Is(err, services.ErrInvalidAidRequestDate),
			errors.Is(err, services.ErrInvalidNeededByDate):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrAidRequestCannotBeEdited):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToUpdateAidRequest,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrAidRequestUpdatedSuccessfully,
		"aid_request": gin.H{
			"id":               aidRequest.ID,
			"person_id":        aidRequest.PersonID,
			"aid_type":         aidRequest.AidType,
			"priority":         aidRequest.Priority,
			"title":            aidRequest.Title,
			"description":      aidRequest.Description,
			"requested_amount": aidRequest.RequestedAmount,
			"currency":         aidRequest.Currency,
			"request_date":     aidRequest.RequestDate,
			"needed_by":        aidRequest.NeededBy,
			"status":           aidRequest.Status,
			"updated_at":       aidRequest.UpdatedAt,
		},
	})
}
func (h *AidRequestHandler) Review(c *gin.Context) {
	aidRequestID := c.Param("id")

	var request models.ReviewAidRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidReviewInfo,
			"error":   err.Error(),
		})
		return
	}

	value, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return
	}

	currentUser, ok := value.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return
	}

	aidRequest, err :=
		h.aidRequestService.ReviewAidRequest(
			aidRequestID,
			request,
			currentUser.ID,
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			services.ErrInvalidAidRequestID,
		):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidAidRequestID,
			})

		case errors.Is(
			err,
			repository.ErrAidRequestNotFound,
		):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrAidRequestNotFound,
			})

		case errors.Is(
			err,
			services.ErrInvalidAidStatus,
		),
			errors.Is(
				err,
				services.ErrInvalidAidStatusTransition,
			),
			errors.Is(
				err,
				services.ErrApprovedAmountRequired,
			),
			errors.Is(
				err,
				services.ErrApprovedAmountTooHigh,
			):
			c.JSON(
				http.StatusUnprocessableEntity,
				gin.H{
					"success": false,
					"message": err.Error(),
				},
			)

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to review aid request",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrAidRequestReviewedSuccessfully,
		"aid_request": gin.H{
			"id":               aidRequest.ID,
			"status":           aidRequest.Status,
			"requested_amount": aidRequest.RequestedAmount,
			"approved_amount":  aidRequest.ApprovedAmount,
			"review_notes":     aidRequest.ReviewNotes,
			"reviewed_by_id":   aidRequest.ReviewedByID,
			"reviewed_at":      aidRequest.ReviewedAt,
		},
	})
}
func (h *AidRequestHandler) Cancel(c *gin.Context) {
	aidRequestID := c.Param("id")

	var request models.CancelAidRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrCancellationReasonRequired,
		})
		return
	}

	value, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return
	}

	currentUser, ok := value.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return
	}

	aidRequest, err := h.aidRequestService.CancelAidRequest(
		aidRequestID,
		request.Reason,
		currentUser.ID,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAidRequestID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidAidRequestID,
			})

		case errors.Is(err, repository.ErrAidRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrAidRequestNotFound,
			})

		case errors.Is(err, services.ErrAidRequestCannotBeCancelled):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToCancelAidRequest,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrAidRequestCancelledSuccessfully,
		"aid_request": gin.H{
			"id":             aidRequest.ID,
			"status":         aidRequest.Status,
			"review_notes":   aidRequest.ReviewNotes,
			"reviewed_by_id": aidRequest.ReviewedByID,
			"reviewed_at":    aidRequest.ReviewedAt,
		},
	})
}
func (h *AidRequestHandler) Delete(c *gin.Context) {
	aidRequestID := c.Param("id")

	err := h.aidRequestService.DeleteAidRequest(aidRequestID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAidRequestID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidAidRequestID,
			})

		case errors.Is(err, repository.ErrAidRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrAidRequestNotFound,
			})

		case errors.Is(err, services.ErrAidRequestCannotBeDeleted):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": constants.ErrAidRequestCannotBeDeleted,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToDeleteAidRequest,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrAidRequestDeletedSuccessfully,
	})
}
