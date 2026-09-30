package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// RegisterReportRoutes wires every report area from the authoritative
// handlers.ReportRegistry:
//
//	HTML : /reports/page                 (landing)
//	       /reports/{slug}/page          (report page, HTML)
//	JSON : /api/reports/{slug}           (structured JSON API)
//	PDF  : /reports/{slug}/pdf           (PDF download, application/pdf)
//
// Authorization is enforced here (backend), never only in the UI:
//   - operational reports: Super Admin, Admin, Manager, Staff
//   - platform reports (users, revenue, account-status, audit-log,
//     system-alerts): Super Admin, Admin only
func RegisterReportRoutes(
	router *gin.Engine,
	handler *handlers.ReportHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	reports := router.Group("/")
	reports.Use(authMiddleware.RequireAuth())
	reports.Use(middleware.RequireAnyRole(
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RoleManager,
		models.RoleStaff,
	))

	// Landing page — opens the HTML reports catalogue (never JSON).
	reports.GET("/reports/page", handler.LandingPage)

	for _, def := range handlers.ReportRegistry() {
		if def.PlatformOnly {
			platform := reports.Group("")
			platform.Use(middleware.RequireAnyRole(
				models.RoleSuperAdmin,
				models.RoleAdmin,
			))
			registerReportArea(platform, handler, def)
			continue
		}

		registerReportArea(reports, handler, def)
	}
}

// registerReportArea registers the three consistent report surfaces for one
// area: HTML page, JSON API and PDF download.
func registerReportArea(
	group *gin.RouterGroup,
	handler *handlers.ReportHandler,
	def handlers.ReportDef,
) {
	group.GET("/reports/"+def.Slug+"/page", handler.PageFor(def))
	group.GET("/api/reports/"+def.Slug, handler.JSONFor(def))
	group.GET("/reports/"+def.Slug+"/pdf", handler.PDFFor(def))
}
