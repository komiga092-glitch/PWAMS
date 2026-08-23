package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterLoanRoutes(
	router *gin.Engine,
	loanHandler *handlers.LoanHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	loans := router.Group("/loans")

	loans.Use(authMiddleware.RequireAuth())
	loans.Use(middleware.RequireAnyRole(
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RoleStaff,
	))

	loans.GET("/page", loanHandler.Page)

	loans.POST("", loanHandler.Create)
	loans.GET("", loanHandler.List)
	loans.GET("/:id", loanHandler.GetByID)
	loans.PATCH("/:id/review", middleware.RequireAnyRole(
		models.RoleSuperAdmin,
		models.RoleAdmin,
	), loanHandler.Review)
}
