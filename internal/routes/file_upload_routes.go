package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

const maxUploadRequestSize int64 = 2*1024*1024 + 64*1024

func RegisterFileUploadRoutes(
	router *gin.Engine,
	fileUploadHandler *handlers.FileUploadHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	files := router.Group("/files")

	files.Use(authMiddleware.RequireAuth())
	files.Use(middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin, models.RoleStaff))

	files.GET("/page", fileUploadHandler.Page)
	files.GET("", fileUploadHandler.List)
	files.GET("/reconciliation", middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin), fileUploadHandler.Reconcile)
	files.POST("/upload", limitUploadRequest(), fileUploadHandler.Upload)
	files.GET("/:id", fileUploadHandler.Download)
	files.DELETE("/:id", fileUploadHandler.Delete)
}

func limitUploadRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxUploadRequestSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"message": "Upload request must not exceed 2 MB plus multipart overhead",
			})
			return
		}

		c.Request.Body = http.MaxBytesReader(
			c.Writer,
			c.Request.Body,
			maxUploadRequestSize,
		)
		c.Next()
	}
}
