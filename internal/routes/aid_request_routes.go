package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterAidRequestRoutes(
	router *gin.Engine,
	aidRequestHandler *handlers.AidRequestHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	aidRequests := router.Group("/aid-requests")

	aidRequests.Use(authMiddleware.RequireAuth())

	aidRequests.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
	)

	aidRequests.GET("", aidRequestHandler.List)
	aidRequests.GET("/:id", aidRequestHandler.GetByID)
	aidRequests.POST("", aidRequestHandler.Create)
	aidRequests.PUT("/:id", aidRequestHandler.Update)
	aidRequests.PATCH(
		"/:id/review",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		aidRequestHandler.Review,
	)
	aidRequests.PATCH(
		"/:id/cancel",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
		aidRequestHandler.Cancel,
	)

	aidRequests.DELETE(
		"/:id",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		aidRequestHandler.Delete,
	)
}
