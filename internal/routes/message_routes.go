package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterMessageRoutes(
	router *gin.Engine,
	messageHandler *handlers.MessageHandler,
	recipientHandler *handlers.MessageRecipientHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	messages := router.Group("/messages")
	messages.Use(authMiddleware.RequireAuth())

	messages.GET("/page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", handlers.PageData(c, gin.H{
			"page_template": "messages_content",
			"title":         "Messages",
		}))
	})
	messages.GET("/recipients", recipientHandler.List)
	messages.GET("", messageHandler.List)
	messages.GET("/inbox", messageHandler.Inbox)
	messages.GET("/sent", messageHandler.Sent)
	messages.GET("/unread", messageHandler.Unread)
	messages.GET("/unread/count", messageHandler.UnreadCount)
	messages.GET("/:id", messageHandler.GetByID)
	messages.POST("", messageHandler.Create)
	messages.PATCH("/:id/read", messageHandler.MarkAsRead)
	messages.DELETE("/:id", messageHandler.Delete)
}
