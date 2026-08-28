package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// writeAudit records financial-record changes (FR-16 / NFR-09).
// Failures are logged-and-ignored: auditing must not block CRUD.
func (h *RevenueHandler) writeAudit(
	c *gin.Context,
	userID interface{ String() string },
	action string,
	entityID string,
	details string,
) {
	if h.auditLogService == nil {
		return
	}
	id := ""
	if userID != nil {
		id = userID.String()
	}
	_ = h.auditLogService.Create(
		id,
		action,
		"revenue_records",
		entityID,
		details,
		c.ClientIP(),
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
	record, err := h.service.Create(request, user.ID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeAudit(c, user.ID, "CREATE", record.ID.String(),
		fmt.Sprintf("type=%s category=%s amount=%s", record.RecordType, record.Category, record.Amount.String()))
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
	record, err := h.service.Update(c.Param("id"), request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	h.writeAudit(c, user.ID, "UPDATE", record.ID.String(),
		fmt.Sprintf("type=%s category=%s amount=%s", record.RecordType, record.Category, record.Amount.String()))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Revenue record updated successfully", "data": record})
}

func (h *RevenueHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(id); err != nil {
		h.writeError(c, err)
		return
	}
	if user, ok := getCurrentUser(c); ok {
		h.writeAudit(c, user.ID, "DELETE", id, "")
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
