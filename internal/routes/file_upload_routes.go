package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterFileUploadRoutes(
	router *gin.Engine,
	fileUploadHandler *handlers.FileUploadHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	files := router.Group("/files")

	files.Use(authMiddleware.RequireAuth())

	files.POST("/upload", fileUploadHandler.Upload)
	files.GET("/:id", fileUploadHandler.Download)
	files.DELETE("/:id", fileUploadHandler.Delete)
}
