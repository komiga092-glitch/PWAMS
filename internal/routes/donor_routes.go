package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterDonorRoutes(
	router *gin.Engine,
	donorHandler *handlers.DonorHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	donors := router.Group("/donors")

	donors.Use(authMiddleware.RequireAuth())

	donors.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
	)

	donors.GET("", donorHandler.List)
	donors.GET("/:id", donorHandler.GetByID)
	donors.POST("", donorHandler.Create)
	donors.PUT("/:id", donorHandler.Update)
	donors.PATCH("/:id/status", donorHandler.UpdateStatus)

	donors.DELETE(
		"/:id",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		donorHandler.Delete,
	)
}
