package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type AidRequestHandler struct {
	aidRequestService *services.AidRequestService
	auditLogService   *services.AuditLogService
}

func NewAidRequestHandler(
	aidRequestService *services.AidRequestService,
	auditLogService *services.AuditLogService,
) *AidRequestHandler {
	return &AidRequestHandler{
		aidRequestService: aidRequestService,
		auditLogService:   auditLogService,
	}
}

func (h *AidRequestHandler) Page(c *gin.Context) {
	var query models.AidRequestListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{"page_template": "aid_requests_content", "title": "Aid Requests", "data": []gin.H{}, "error": "Invalid query parameters"}))
		return
	}
	requests, _, _, _, err := h.aidRequestService.ListAidRequests(query)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{"page_template": "aid_requests_content", "title": "Aid Requests", "data": []gin.H{}, "error": "Unable to retrieve aid requests"}))
		return
	}
	items := make([]gin.H, 0, len(requests))
	for _, request := range requests {
		items = append(items, gin.H{"ID": request.ID, "PersonID": request.PersonID, "PersonName": request.Person.FullName, "RequestType": request.AidType, "Amount": request.RequestedAmount, "RequestedAt": request.RequestDate, "Status": request.Status})
	}
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{"page_template": "aid_requests_content", "title": "Aid Requests", "data": items, "search": query.Search, "status": query.Status}))
}

func (h *AidRequestHandler) Create(c *gin.Context) {
	var request models.CreateAidRequest

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidAidRequestInfo,
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

	var aidRequest *models.AidRequest

	// The aid request and its mandatory audit entry are committed together
	// or not at all (SRS FR-16 / NFR-09): a request can never exist without
	// its audit trail, and an audit failure rolls the create back.
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		created, createErr := h.aidRequestService.WithTx(tx).CreateAidRequest(
			request,
			currentUser.ID,
		)
		if createErr != nil {
			return createErr
		}

		aidRequest = created

		return h.auditLogService.Audit(
			tx,
			currentUser.ID.String(),
			"CREATE",
			"aid_requests",
			created.ID.String(),
			"Aid request created successfully",
		)
	})

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to create aid request",
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: "Invalid person ID"},
			errorResponseMapping{err: repository.ErrPersonNotFound, status: http.StatusNotFound, message: "Person not found"},
			errorResponseMapping{err: services.ErrInvalidAidType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidPriority, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidRequestDate, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidNeededByDate, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidRequestAmount, status: http.StatusUnprocessableEntity, message: err.Error()},
		)
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
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to retrieve aid requests",
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: constants.ErrInvalidPersonID},
			errorResponseMapping{err: services.ErrInvalidAidType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidPriority, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidDateRange, status: http.StatusUnprocessableEntity, message: err.Error()},
		)
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
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to retrieve aid request",
			errorResponseMapping{err: services.ErrInvalidAidRequestID, status: http.StatusBadRequest, message: constants.ErrInvalidAidRequestID},
			errorResponseMapping{err: repository.ErrAidRequestNotFound, status: http.StatusNotFound, message: "Aid request not found"},
		)
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

	var aidRequest *models.AidRequest

	// The update and its mandatory audit entry are committed together or
	// not at all (SRS FR-16 / NFR-09).
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		updated, updateErr := h.aidRequestService.WithTx(tx).UpdateAidRequest(
			aidRequestID,
			request,
		)
		if updateErr != nil {
			return updateErr
		}

		aidRequest = updated

		return h.auditLogService.Audit(
			tx,
			currentUser.ID.String(),
			"UPDATE",
			"aid_requests",
			updated.ID.String(),
			"Aid request updated successfully",
		)
	})

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToUpdateAidRequest,
			errorResponseMapping{err: services.ErrInvalidAidRequestID, status: http.StatusBadRequest, message: constants.ErrInvalidAidRequestID},
			errorResponseMapping{err: repository.ErrAidRequestNotFound, status: http.StatusNotFound, message: constants.ErrAidRequestNotFound},
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: constants.ErrInvalidPersonID},
			errorResponseMapping{err: repository.ErrPersonNotFound, status: http.StatusNotFound, message: constants.ErrPersonNotFound},
			errorResponseMapping{err: services.ErrInvalidAidType, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidPriority, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidRequestDate, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidNeededByDate, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrAidRequestCannotBeEdited, status: http.StatusConflict, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidRequestAmount, status: http.StatusUnprocessableEntity, message: err.Error()},
		)
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

	var aidRequest *models.AidRequest

	// The review decision and its mandatory audit entry are committed
	// together or not at all (SRS FR-16 / NFR-09).
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		reviewed, reviewErr :=
			h.aidRequestService.WithTx(tx).ReviewAidRequest(
				aidRequestID,
				request,
				currentUser.ID,
			)
		if reviewErr != nil {
			return reviewErr
		}

		aidRequest = reviewed

		return h.auditLogService.Audit(
			tx,
			currentUser.ID.String(),
			"REVIEW",
			"aid_requests",
			reviewed.ID.String(),
			fmt.Sprintf("status=%s approved_amount=%s", reviewed.Status, reviewed.ApprovedAmount.String()),
		)
	})

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, "Unable to review aid request",
			errorResponseMapping{err: services.ErrInvalidAidRequestID, status: http.StatusBadRequest, message: constants.ErrInvalidAidRequestID},
			errorResponseMapping{err: repository.ErrAidRequestNotFound, status: http.StatusNotFound, message: constants.ErrAidRequestNotFound},
			errorResponseMapping{err: services.ErrInvalidAidStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidAidStatusTransition, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrApprovedAmountRequired, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrApprovedAmountTooHigh, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrCannotReviewOwnSubmission, status: http.StatusForbidden, message: err.Error()},
		)
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

	var aidRequest *models.AidRequest

	// The cancellation and its mandatory audit entry are committed together
	// or not at all (SRS FR-16 / NFR-09).
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		cancelled, cancelErr := h.aidRequestService.WithTx(tx).CancelAidRequest(
			aidRequestID,
			request.Reason,
			currentUser.ID,
		)
		if cancelErr != nil {
			return cancelErr
		}

		aidRequest = cancelled

		return h.auditLogService.Audit(
			tx,
			currentUser.ID.String(),
			"CANCEL",
			"aid_requests",
			cancelled.ID.String(),
			"Aid request cancelled successfully",
		)
	})

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToCancelAidRequest,
			errorResponseMapping{err: services.ErrInvalidAidRequestID, status: http.StatusBadRequest, message: constants.ErrInvalidAidRequestID},
			errorResponseMapping{err: repository.ErrAidRequestNotFound, status: http.StatusNotFound, message: constants.ErrAidRequestNotFound},
			errorResponseMapping{err: services.ErrAidRequestCannotBeCancelled, status: http.StatusConflict, message: err.Error()},
		)
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

	// The deletion and its mandatory audit entry are committed together or
	// not at all (SRS FR-16 / NFR-09).
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		if deleteErr := h.aidRequestService.WithTx(tx).DeleteAidRequest(aidRequestID); deleteErr != nil {
			return deleteErr
		}

		return h.auditLogService.Audit(
			tx,
			currentUser.ID.String(),
			"DELETE",
			"aid_requests",
			aidRequestID,
			"Aid request deleted successfully",
		)
	})
	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToDeleteAidRequest,
			errorResponseMapping{err: services.ErrInvalidAidRequestID, status: http.StatusBadRequest, message: constants.ErrInvalidAidRequestID},
			errorResponseMapping{err: repository.ErrAidRequestNotFound, status: http.StatusNotFound, message: constants.ErrAidRequestNotFound},
			errorResponseMapping{err: services.ErrAidRequestCannotBeDeleted, status: http.StatusConflict, message: constants.ErrAidRequestCannotBeDeleted},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrAidRequestDeletedSuccessfully,
	})
}
