package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterLoanRepaymentRoutes(
	router *gin.Engine,
	loanRepaymentHandler *handlers.LoanRepaymentHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	repayments := router.Group("/loan-repayments")

	repayments.Use(authMiddleware.RequireAuth())
	repayments.Use(middleware.RequireAnyRole(
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RoleStaff,
	))

	repayments.GET("/page", loanRepaymentHandler.Page)

	repayments.POST("", loanRepaymentHandler.Create)
	repayments.GET("", loanRepaymentHandler.List)
	repayments.GET("/:id", loanRepaymentHandler.GetByID)
	repayments.PATCH("/:id/pay", loanRepaymentHandler.Pay)
	repayments.PATCH("/:id/cancel", loanRepaymentHandler.Cancel)
}
