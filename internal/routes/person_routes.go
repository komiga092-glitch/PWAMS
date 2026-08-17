package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterPersonRoutes(
	router *gin.Engine,
	personHandler *handlers.PersonHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	persons := router.Group("/persons")

	persons.Use(authMiddleware.RequireAuth())

	persons.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
	)

	// Persons UI page
	persons.GET("/page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", gin.H{
			"page_template": "persons_content",
			"title":         "Care Seekers",
		})
	})

	// Persons API
	persons.GET("", personHandler.List)
	persons.GET("/:id", personHandler.GetByID)
	persons.POST("", personHandler.Create)
	persons.PUT("/:id", personHandler.Update)
	persons.PATCH("/:id/status", personHandler.UpdateStatus)

	persons.DELETE(
		"/:id",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		personHandler.Delete,
	)
}
