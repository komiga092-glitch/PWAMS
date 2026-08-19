package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterStudentRoutes(
	router *gin.Engine,
	studentHandler *handlers.StudentHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	students := router.Group("/students")

	students.Use(authMiddleware.RequireAuth())

	students.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
	)

	// HTML page

	// API endpoints
	students.GET("", studentHandler.List)
	students.GET("/page", studentHandler.Page)

students.GET("/:id", studentHandler.GetByID)
	students.POST("", studentHandler.Create)
	students.PUT("/:id", studentHandler.Update)
	students.PATCH("/:id/status", studentHandler.UpdateStatus)

	students.DELETE(
		"/:id",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		studentHandler.Delete,
	)
}
