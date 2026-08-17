package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterDashboardRoutes(
	router *gin.Engine,
	dashboardHandler *handlers.DashboardHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	dashboard := router.Group("/dashboard")

	dashboard.Use(authMiddleware.RequireAuth())

	dashboard.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
	)

	dashboard.GET("/stats", dashboardHandler.GetStats)
}
