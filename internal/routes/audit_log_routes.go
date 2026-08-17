package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterAuditLogRoutes(
	router *gin.Engine,
	handler *handlers.AuditLogHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	protected := router.Group("/")
	protected.Use(
		authMiddleware.RequireAuth(),
	)

	protected.GET(
		"/audit-logs/page",
		func(c *gin.Context) {
			c.HTML(http.StatusOK, "base", gin.H{
				"page_template": "audit_logs_content",
				"title":         "Audit Logs",
			})
		},
	)

	protected.GET(
		"/audit-logs",
		handler.List,
	)

	protected.GET(
		"/audit-logs/:id",
		handler.GetByID,
	)
	
}
