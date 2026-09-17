package routes

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// RegisterSystemSettingsRoutes exposes the read-only System Settings page.
// The route is gated by system.settings.view — a protected permission held
// exclusively by the Super Admin role — so the page is invisible and
// unreachable for every other actor.
//
// Fail-closed rule: a nil permission service leaves the page unregistered
// instead of exposing the system overview to any authenticated user.
func RegisterSystemSettingsRoutes(
	router *gin.Engine,
	handler *handlers.SystemSettingsHandler,
	authMiddleware *middleware.AuthMiddleware,
	permissionService *services.PermissionService,
) {
	if permissionService == nil {
		log.Printf("routes: permission service unavailable — /system-settings page NOT registered (fail-closed)")
		return
	}

	group := router.Group("/system-settings")
	group.Use(authMiddleware.RequireAuth())

	group.Use(middleware.LoadPermissions(permissionService))
	group.GET("/page",
		middleware.RequirePermission(permissionService, "system.settings.view"),
		handler.Page)
}
