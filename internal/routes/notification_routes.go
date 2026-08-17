package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterNotificationRoutes(
	router *gin.Engine,
	notificationHandler *handlers.NotificationHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	notifications := router.Group("/notifications")

	notifications.Use(authMiddleware.RequireAuth())

	notifications.GET("/page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", gin.H{
			"page_template": "notifications_content",
			"title":         "Notifications",
		})
	})

	notifications.GET("", notificationHandler.List)
	notifications.GET("/:id", notificationHandler.GetByID)
	notifications.POST("", notificationHandler.Create)
	notifications.PATCH("/:id/read", notificationHandler.MarkAsRead)
	notifications.DELETE("/:id", notificationHandler.Delete)
}
