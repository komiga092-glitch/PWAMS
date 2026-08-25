package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterNotificationRoutes(
	router *gin.Engine,
	notificationHandler *handlers.NotificationHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	notifications := router.Group("/notifications")

	notifications.Use(authMiddleware.RequireAuth())

	notifications.GET("/page", notificationHandler.Page)

	notifications.GET("", notificationHandler.List)
	notifications.GET("/:id", notificationHandler.GetByID)
	notifications.POST("", middleware.RequireAnyRole(
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RoleStaff,
	), notificationHandler.Create)
	notifications.PATCH("/:id/read", notificationHandler.MarkAsRead)
	notifications.DELETE("/:id", notificationHandler.Delete)
}
