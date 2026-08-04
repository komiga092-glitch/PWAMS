package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
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
}
