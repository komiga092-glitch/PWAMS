package routes

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// RegisterPermissionRoutes exposes the permission-management API. The route
// group itself is restricted to Super Admin and Admin; fine-grained
// permission checks (permission_management.view / .edit) and per-role
// escalation guards are enforced inside the handlers and service layer.
func RegisterPermissionRoutes(
	router *gin.Engine,
	permissionHandler *handlers.PermissionHandler,
	authMiddleware *middleware.AuthMiddleware,
	permissionService *services.PermissionService,
) {
	// FAIL-CLOSED (security): every /permissions endpoint is privileged.
	// Without a working permission service the granular gates cannot be
	// evaluated, so the module is NOT registered. The role gate alone
	// (Super Admin / Admin) is not a substitute for the permission checks.
	if permissionService == nil {
		log.Printf("routes: permission service unavailable — /permissions module NOT registered (fail-closed)")
		return
	}

	group := router.Group("/permissions")
	group.Use(authMiddleware.RequireAuth())
	group.Use(middleware.LoadPermissions(permissionService))
	group.Use(middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin))

	// Permission catalog & role matrix (view)
	group.GET("/page", middleware.RequirePermission(permissionService, "permission_management.view"), permissionHandler.Page)
	group.GET("", middleware.RequirePermission(permissionService, "permission_management.view"), permissionHandler.ListAll)
	group.GET("/roles", middleware.RequirePermission(permissionService, "permission_management.view"), permissionHandler.ListRoles)
	group.GET("/roles/:name", middleware.RequirePermission(permissionService, "permission_management.view"), permissionHandler.GetRolePermissions)

	// Role permission edits (edit permission required)
	group.PUT("/roles/:name", middleware.RequirePermission(permissionService, "permission_management.edit"), permissionHandler.UpdateRolePermissions)

	// User-specific overrides
	group.GET("/users/:id", middleware.RequirePermission(permissionService, "permission_management.view"), permissionHandler.GetUserOverrides)
	group.PUT("/users/:id", middleware.RequirePermission(permissionService, "permission_management.edit"), permissionHandler.UpdateUserOverrides)
}
