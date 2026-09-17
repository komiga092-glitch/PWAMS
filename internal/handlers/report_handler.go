package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type ReportHandler struct {
	reportService    *services.ReportService
	reportPDFService *services.ReportPDFService
}

func NewReportHandler(
	reportService *services.ReportService,
	reportPDFService *services.ReportPDFService,
) *ReportHandler {
	return &ReportHandler{
		reportService:    reportService,
		reportPDFService: reportPDFService,
	}
}

func (h *ReportHandler) Page(c *gin.Context) {
	dashboard, dashboardErr := h.reportService.GetDashboardReport()
	donations, donationsErr := h.reportService.GetDonationReport()
	aidRequests, aidErr := h.reportService.GetAidRequestReport()
	if dashboardErr != nil || donationsErr != nil || aidErr != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"page_template": "reports_content",
			"title":         "Reports",
			"error":         "Unable to retrieve reports",
		}))
		return
	}
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "reports_content",
		"title":         "Reports",
		"dashboard":     dashboard,
		"donations":     donations,
		"aid_requests":  aidRequests,
	}))
}

func (h *ReportHandler) GetDashboardReport(c *gin.Context) {
	report, err := h.reportService.GetDashboardReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve dashboard report"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Dashboard report retrieved successfully", "data": report})
}

func (h *ReportHandler) GetDonationReport(c *gin.Context) {
	report, err := h.reportService.GetDonationReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve donation report"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Donation report retrieved successfully", "data": report})
}

func (h *ReportHandler) GetAidRequestReport(c *gin.Context) {
	report, err := h.reportService.GetAidRequestReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve aid request report"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Aid request report retrieved successfully", "data": report})
}

func (h *ReportHandler) DownloadSummaryPDF(c *gin.Context) {
	dashboard, err := h.reportService.GetDashboardReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve dashboard report"})
		return
	}
	donations, err := h.reportService.GetDonationReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve donation report"})
		return
	}
	aidRequests, err := h.reportService.GetAidRequestReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve aid request report"})
		return
	}

	pdfBytes, err := h.reportPDFService.RenderSummaryReport(dashboard, donations, aidRequests)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to generate report PDF"})
		return
	}

	filename := "pwams-summary-report-" + time.Now().Format("2006-01-02") + ".pdf"
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
