package routes

import (
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

	persons.POST("", personHandler.Create)
}
