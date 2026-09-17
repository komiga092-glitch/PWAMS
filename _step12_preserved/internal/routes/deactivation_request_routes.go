package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// RegisterDeactivationRequestRoutes exposes the Super Admin deactivation
// approval workflow. Only Super Admins may create or respond to requests.
func RegisterDeactivationRequestRoutes(
	router *gin.Engine,
	deactivationRequestHandler *handlers.DeactivationRequestHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	group := router.Group("/super-admins/deactivations")
	group.Use(authMiddleware.RequireAuth())
	group.Use(middleware.RequireRole(models.RoleSuperAdmin))

	group.GET("", deactivationRequestHandler.List)
	group.POST("", deactivationRequestHandler.Create)
	group.POST("/:id/accept", deactivationRequestHandler.Accept)
	group.POST("/:id/reject", deactivationRequestHandler.Reject)
}
