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

	students.POST("", studentHandler.Create)
}
