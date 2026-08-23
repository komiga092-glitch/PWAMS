package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type RevenueHandler struct{ service *services.RevenueService }

func NewRevenueHandler(service *services.RevenueService) *RevenueHandler {
	return &RevenueHandler{service: service}
}

func (h *RevenueHandler) Page(c *gin.Context) {
	records, _, _, _, err := h.service.List(models.RevenueListQuery{Page: 1, PageSize: 100})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", gin.H{"page_template": "revenue_content", "title": "Revenue Management", "error": "Unable to retrieve revenue records"})
		return
	}
	c.HTML(http.StatusOK, "base", gin.H{"page_template": "revenue_content", "title": "Revenue Management", "records": records})
}

func (h *RevenueHandler) Create(c *gin.Context) {
	var request models.CreateRevenueRecordRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid revenue record", "error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid revenue record", "error": err.Error()})
		return
	}
	record, err := h.service.Update(c.Param("id"), request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Revenue record updated successfully", "data": record})
}

func (h *RevenueHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Param("id")); err != nil {
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
