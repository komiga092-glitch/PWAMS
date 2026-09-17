package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/services"
)

// SystemAlertHandler exposes the privileged system-alert inbox. The routes
// are protected by the protected system.alerts.view / system.alerts.manage
// permissions (Super Admin only by default) — technical details about system
// failures are never exposed to ordinary roles.
type SystemAlertHandler struct {
	alertService *services.SystemAlertService
}

func NewSystemAlertHandler(alertService *services.SystemAlertService) *SystemAlertHandler {
	return &SystemAlertHandler{alertService: alertService}
}

// Page renders the system alerts overview.
func (h *SystemAlertHandler) Page(c *gin.Context) {
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":         "System Alerts",
		"page_template": "system_alerts_content",
	}))
}

// List returns the alert inbox as JSON, newest first. Supports status,
// severity, category and search filters with server-side pagination.
func (h *SystemAlertHandler) List(c *gin.Context) {
	status := strings.TrimSpace(c.Query("status"))
	severity := strings.TrimSpace(c.Query("severity"))
	category := strings.TrimSpace(c.Query("category"))
	search := strings.TrimSpace(c.Query("search"))
	limit := parseAlertQueryInt(c.Query("limit"), 100)
	offset := parseAlertQueryInt(c.Query("offset"), 0)

	// Accept the legacy "level" query parameter as an alias for severity so
	// existing bookmarks/integrations keep working.
	if severity == "" && c.Query("level") != "" {
		severity = c.Query("level")
	}

	alerts, total, err := h.alertService.List(status, severity, category, search, int32(limit), int32(offset))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve system alerts",
		})
		return
	}

	stats, statsErr := h.alertService.Stats()
	if statsErr != nil {
		stats = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"alerts":  alerts,
		"total":   total,
		"stats":   stats,
	})
}

// GetByID returns one alert for the card-based detail view.
func (h *SystemAlertHandler) GetByID(c *gin.Context) {
	alert, err := h.alertService.ByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "System alert not found",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "alert": alert})
}

// Stats returns the summary card counts.
func (h *SystemAlertHandler) Stats(c *gin.Context) {
	stats, err := h.alertService.Stats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve system alert statistics",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "stats": stats})
}

// Read marks an alert as read without resolving it.
func (h *SystemAlertHandler) Read(c *gin.Context) {
	if err := h.alertService.MarkRead(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "System alert not found",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "System alert marked as read"})
}

// Resolve marks an open alert as resolved by the authenticated operator.
func (h *SystemAlertHandler) Resolve(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	if err := h.alertService.Resolve(c.Param("id"), currentUser.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "System alert not found or already resolved",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "System alert resolved",
	})
}

func parseAlertQueryInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return fallback
	}
	if v > 500 {
		return 500
	}
	return v
}
