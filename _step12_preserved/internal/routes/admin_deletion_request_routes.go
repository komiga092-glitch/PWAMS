package routes

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// RegisterAdminDeletionRequestRoutes exposes the supervised Admin account
// deletion workflow under the Admin Management module. Every endpoint is
// gated by its dedicated permission:
//
//	POST /admins/deletion-requests          -> admin.delete.request
//	GET  /admins/deletion-requests          -> admin.view (module visibility)
//	POST /admins/deletion-requests/:id/approve -> admin.delete.approve
//	POST /admins/deletion-requests/:id/reject  -> admin.delete.reject
//	POST /admins/deletion-requests/:id/cancel  -> admin.delete.request
//
// The service layer enforces the role hierarchy (only Admin accounts are
// ever targeted; an Admin can never reach a Super Admin account) and the
// four-eyes rule (the approver/rejecter must differ from the requester and
// from the target) independently of these gates.
//
// Fail-closed rule: a nil permission service leaves the workflow unregistered
// rather than exposing it to any authenticated user (fail-open behaviour is
// never acceptable for privileged endpoints).
func RegisterAdminDeletionRequestRoutes(
	router *gin.Engine,
	deletionRequestHandler *handlers.AdminDeletionRequestHandler,
	authMiddleware *middleware.AuthMiddleware,
	permSvc *services.PermissionService,
) {
	if permSvc == nil {
		log.Printf("routes: permission service unavailable — /admins/deletion-requests workflow NOT registered (fail-closed)")
		return
	}

	group := router.Group("/admins/deletion-requests")
	group.Use(authMiddleware.RequireAuth())
	group.Use(middleware.LoadPermissions(permSvc))

	group.GET("",
		middleware.RequirePermission(permSvc, "admin.view"),
		deletionRequestHandler.List)
	group.POST("",
		middleware.RequirePermission(permSvc, "admin.delete.request"),
		deletionRequestHandler.Create)
	group.POST("/:id/approve",
		middleware.RequirePermission(permSvc, "admin.delete.approve"),
		deletionRequestHandler.Approve)
	group.POST("/:id/reject",
		middleware.RequirePermission(permSvc, "admin.delete.reject"),
		deletionRequestHandler.Reject)
	group.POST("/:id/cancel",
		middleware.RequirePermission(permSvc, "admin.delete.request"),
		deletionRequestHandler.Cancel)
}
