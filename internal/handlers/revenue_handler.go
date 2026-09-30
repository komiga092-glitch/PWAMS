package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// auditTx records financial-record changes (FR-16 / NFR-09) inside the
// caller's transaction. Financial records are mandatory audit events, so a
// failure is returned to the caller and rolls the business write back: a
// revenue record must never be committed without its audit trail.
func (h *RevenueHandler) auditTx(
	tx *gorm.DB,
	userID interface{ String() string },
	action string,
	entityID string,
	details string,
) error {
	id := ""
	if userID != nil {
		id = userID.String()
	}
	return h.auditLogService.Audit(
		tx,
		id,
		action,
		"revenue_records",
		entityID,
		details,
	)
}

type RevenueHandler struct {
	service         *services.RevenueService
	auditLogService *services.AuditLogService
}

func NewRevenueHandler(service *services.RevenueService, auditLogService *services.AuditLogService) *RevenueHandler {
	return &RevenueHandler{service: service, auditLogService: auditLogService}
}

func (h *RevenueHandler) Page(c *gin.Context) {
	records, _, _, _, err := h.service.List(models.RevenueListQuery{Page: 1, PageSize: 100})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{"page_template": "revenue_content", "title": "Revenue Management", "error": "Unable to retrieve revenue records"}))
		return
	}
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{"page_template": "revenue_content", "title": "Revenue Management", "records": records}))
}

func (h *RevenueHandler) Create(c *gin.Context) {
	var request models.CreateRevenueRecordRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid revenue record"})
		return
	}
	user, ok := getCurrentUser(c)
	if !ok {
		return
	}

	var record *models.RevenueRecord

	// The revenue record and its mandatory audit entry are committed together
	// or not at all.
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		created, createErr := h.service.WithTx(tx).Create(request, user.ID)
		if createErr != nil {
			return createErr
		}

		record = created

		return h.auditTx(tx, user.ID, "CREATE", created.ID.String(),
			fmt.Sprintf("type=%s category=%s amount=%s", created.RecordType, created.Category, created.Amount.String()))
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Revenue record created successfully", "data": record})
}

func (h *RevenueHandler) List(c *gin.Context) {
	var query models.RevenueListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid revenue query"})
		return
	}
	records, total, page, pageSize, err := h.service.List(query)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": records, "pagination": buildPagination(total, page, pageSize)})
}

func (h *RevenueHandler) GetByID(c *gin.Context) {
	record, err := h.service.GetByID(c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": record})
}

func (h *RevenueHandler) Update(c *gin.Context) {
	var request models.UpdateRevenueRecordRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid revenue record"})
		return
	}
	user, ok := getCurrentUser(c)
	if !ok {
		return
	}

	var record *models.RevenueRecord

	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		updated, updateErr := h.service.WithTx(tx).Update(c.Param("id"), request)
		if updateErr != nil {
			return updateErr
		}

		record = updated

		return h.auditTx(tx, user.ID, "UPDATE", updated.ID.String(),
			fmt.Sprintf("type=%s category=%s amount=%s", updated.RecordType, updated.Category, updated.Amount.String()))
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Revenue record updated successfully", "data": record})
}

func (h *RevenueHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	user, ok := getCurrentUser(c)
	if !ok {
		return
	}

	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		if deleteErr := h.service.WithTx(tx).Delete(id); deleteErr != nil {
			return deleteErr
		}

		return h.auditTx(tx, user.ID, "DELETE", id, "")
	})
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Revenue record deleted successfully"})
}

func (h *RevenueHandler) Summaries(c *gin.Context) {
	summaries, err := h.service.Summaries(nowUTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve revenue summaries"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summaries})
}

func nowUTC() time.Time { return time.Now().UTC() }

func (h *RevenueHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, services.ErrInvalidRevenueID), errors.Is(err, services.ErrInvalidRevenueDate):
		status = http.StatusBadRequest
	case errors.Is(err, services.ErrInvalidRevenueType), errors.Is(err, services.ErrInvalidRevenueCategory), errors.Is(err, services.ErrInvalidRevenueAmount):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, services.ErrRevenueReferenceExists):
		status = http.StatusConflict
	case errors.Is(err, repository.ErrRevenueRecordNotFound):
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"success": false, "message": err.Error()})
}
