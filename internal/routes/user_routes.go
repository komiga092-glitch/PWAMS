package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterUserRoutes(
	router *gin.Engine,
	userHandler *handlers.UserHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	users := router.Group("/users")

	users.Use(authMiddleware.RequireAuth())

	users.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
	)

	users.GET("", userHandler.List)
	users.GET("/:id", userHandler.GetByID)
	users.POST("", userHandler.Create)
	users.PUT("/:id", userHandler.Update)
	users.PATCH("/:id/status", userHandler.UpdateStatus)
	users.PATCH("/:id/password", userHandler.ResetPassword)
}
