package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterDonationRoutes(
	router *gin.Engine,
	donationHandler *handlers.DonationHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	donations := router.Group("/donations")

	donations.Use(authMiddleware.RequireAuth())

	donations.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
	)

	donations.GET("", donationHandler.List)
	donations.POST("", donationHandler.Create)
}
