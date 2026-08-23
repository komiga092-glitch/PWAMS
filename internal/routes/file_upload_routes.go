package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterFileUploadRoutes(
	router *gin.Engine,
	fileUploadHandler *handlers.FileUploadHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	files := router.Group("/files")

	files.Use(authMiddleware.RequireAuth())
	files.Use(middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin, models.RoleStaff))

	files.POST("/upload", fileUploadHandler.Upload)
	files.GET("/:id", fileUploadHandler.Download)
	files.DELETE("/:id", fileUploadHandler.Delete)
}
