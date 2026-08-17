package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type AuditLogHandler struct {
	auditLogService *services.AuditLogService
}

func NewAuditLogHandler(
	auditLogService *services.AuditLogService,
) *AuditLogHandler {
	return &AuditLogHandler{
		auditLogService: auditLogService,
	}
}

func (h *AuditLogHandler) List(c *gin.Context) {
	var query models.AuditLogListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid audit log query",
		})
		return
	}

	// Prevent invalid negative values from reaching the repository.
	if query.Page < 1 {
		query.Page = 1
	}

	if query.PageSize < 1 {
		query.PageSize = 20
	}

	if query.PageSize > 100 {
		query.PageSize = 100
	}

	logs, total, page, totalPages, err :=
		h.auditLogService.List(query)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve audit logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Audit logs retrieved successfully",
		"data":    logs,
		"pagination": gin.H{
			"page":        page,
			"page_size":   query.PageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}

func (h *AuditLogHandler) GetByID(c *gin.Context) {
	log, err := h.auditLogService.GetByID(
		c.Param("id"),
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAuditLogID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid audit log ID",
			})

		case errors.Is(err, repository.ErrAuditLogNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Audit log not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve audit log",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Audit log retrieved successfully",
		"data":    log,
	})
}

func (h *AuditLogHandler) Delete(c *gin.Context) {
	err := h.auditLogService.Delete(
		c.Param("id"),
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAuditLogID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid audit log ID",
			})

		case errors.Is(err, repository.ErrAuditLogNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Audit log not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete audit log",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Audit log deleted successfully",
	})
}

// Keep strconv available for future pagination/query handling.
