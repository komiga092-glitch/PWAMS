package routes

import (
	"net/http"

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

	donations.GET("/page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", gin.H{
			"page_template": "donations_content",
			"title":         "Donations",
			"data":          []interface{}{},
		})
	})

	donations.GET("", donationHandler.List)
	donations.GET("/:id", donationHandler.GetByID)
	donations.POST("", donationHandler.Create)
	donations.PUT("/:id", donationHandler.Update)
	donations.PATCH("/:id/status", donationHandler.UpdateStatus)

	donations.DELETE(
		"/:id",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		donationHandler.Delete,
	)
}
