package routes

import (
	"github.com/gin-gonic/gin"
	"net/http"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterAuthRoutes(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	// Public route
	router.POST("/login", authHandler.Login)

	// Authentication required routes
	protected := router.Group("/")
	protected.Use(authMiddleware.RequireAuth())

	protected.GET("/dashboard", authHandler.Dashboard)
	protected.POST("/logout", authHandler.Logout)

	protected.GET(
		"/admin-test",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Admin-level access granted",
			})
		},
	)

}
