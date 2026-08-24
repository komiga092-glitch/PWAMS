package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterSyncRoutes(
	router *gin.Engine,
	handler *handlers.SyncHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	sync := router.Group("/api/v1/sync")

	sync.Use(authMiddleware.RequireAuth())
	sync.Use(middleware.RequireAnyRole(
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RoleStaff,
	))

	sync.POST("/push", handler.Push)
	sync.GET("/pull", handler.Pull)
}
