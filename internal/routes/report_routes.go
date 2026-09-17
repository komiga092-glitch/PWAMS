package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterReportRoutes(
	router *gin.Engine,
	handler *handlers.ReportHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	protected := router.Group("/")
	protected.Use(
		authMiddleware.RequireAuth(),
		middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin),
	)

	protected.GET(
		"/reports/dashboard/page",
		handler.Page,
	)

	protected.GET(
		"/reports/dashboard/pdf",
		handler.DownloadSummaryPDF,
	)

	protected.GET(
		"/reports/dashboard",
		handler.GetDashboardReport,
	)

	protected.GET(
		"/reports/donations",
		handler.GetDonationReport,
	)

	protected.GET(
		"/reports/aid-requests",
		handler.GetAidRequestReport,
	)
}
