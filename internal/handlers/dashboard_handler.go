package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type DashboardHandler struct {
	dashboardService *services.DashboardService
}

func NewDashboardHandler(
	dashboardService *services.DashboardService,
) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
	}
}

// Page renders the dashboard page.
func (h *DashboardHandler) Page(c *gin.Context) {
	stats, err := h.dashboardService.GetStats()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", gin.H{
			"page_template": "dashboard_content",
			"title":         "PWAMS Dashboard",
			"error":         constants.ErrUnableToRetrieveDashboardStats,
		})
		return
	}

	c.HTML(http.StatusOK, "base", gin.H{
		"page_template": "dashboard_content",
		"title":         "PWAMS Dashboard",
		"stats":         stats,
	})
}

// GetStats returns dashboard statistics as JSON.
func (h *DashboardHandler) GetStats(c *gin.Context) {
	stats, err := h.dashboardService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrUnableToRetrieveDashboardStats,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrDashboardStatsRetrievedSuccessfully,
		"data":    stats,
	})
}
