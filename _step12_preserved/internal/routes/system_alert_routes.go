package routes

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// RegisterSystemAlertRoutes exposes the privileged system-alert inbox. Both
// route gates are protected permissions (system.* namespace — Super Admin
// only by default), so critical technical details about system failures are
// never surfaced to ordinary roles.
//
// Fail-closed rule: a nil permission service leaves the module unregistered
// (404) instead of exposing system alert data to any authenticated user.
func RegisterSystemAlertRoutes(
	router *gin.Engine,
	handler *handlers.SystemAlertHandler,
	authMiddleware *middleware.AuthMiddleware,
	permissionService *services.PermissionService,
) {
	if permissionService == nil {
		log.Printf("routes: permission service unavailable — /system-alerts module NOT registered (fail-closed)")
		return
	}

	group := router.Group("/system-alerts")
	group.Use(authMiddleware.RequireAuth())
	group.Use(middleware.LoadPermissions(permissionService))

	group.GET("/page",
		middleware.RequirePermission(permissionService, "system.alerts.view"),
		handler.Page)
	group.GET("/api",
		middleware.RequirePermission(permissionService, "system.alerts.view"),
		handler.List)
	group.GET("/api/stats",
		middleware.RequirePermission(permissionService, "system.alerts.view"),
		handler.Stats)
	group.GET("/api/:id",
		middleware.RequirePermission(permissionService, "system.alerts.view"),
		handler.GetByID)
	group.POST("/api/:id/read",
		middleware.RequirePermission(permissionService, "system.alerts.view"),
		handler.Read)
	group.POST("/api/:id/resolve",
		middleware.RequirePermission(permissionService, "system.alerts.manage"),
		handler.Resolve)
}
