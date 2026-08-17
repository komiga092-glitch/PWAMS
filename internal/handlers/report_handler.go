package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/services"
)

type ReportHandler struct {
	reportService *services.ReportService
}

func NewReportHandler(
	reportService *services.ReportService,
) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
	}
}

func (h *ReportHandler) GetDashboardReport(c *gin.Context) {
	report, err := h.reportService.GetDashboardReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve dashboard report",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Dashboard report retrieved successfully",
		"data":    report,
	})
}

func (h *ReportHandler) GetDonationReport(c *gin.Context) {
	report, err := h.reportService.GetDonationReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve donation report",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Donation report retrieved successfully",
		"data":    report,
	})
}

func (h *ReportHandler) GetAidRequestReport(c *gin.Context) {
	report, err := h.reportService.GetAidRequestReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve aid request report",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Aid request report retrieved successfully",
		"data":    report,
	})
}
