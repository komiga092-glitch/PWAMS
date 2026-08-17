package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterReportRoutes(
	router *gin.Engine,
	handler *handlers.ReportHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	protected := router.Group("/")
	protected.Use(
		authMiddleware.RequireAuth(),
	)

	protected.GET(
		"/reports/dashboard/page",
		func(c *gin.Context) {
			c.HTML(http.StatusOK, "base", gin.H{
				"page_template": "reports_content",
				"title":         "Reports",
			})
		},
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
