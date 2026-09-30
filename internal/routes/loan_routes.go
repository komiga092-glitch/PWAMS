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
		models.RoleManager,
		models.RoleStaff,
	))

	loans.GET("/page", loanHandler.Page)

	loans.POST("", loanHandler.Create)
	loans.GET("", loanHandler.List)
	loans.GET("/:id", loanHandler.GetByID)

	// Loan edit (models.UpdateLoanRequest): the object-level rule — the
	// submitter or a privileged role, and only while the loan is still
	// Pending — is enforced in LoanService.UpdateLoan, so the whole
	// Staff-and-above group shares the route.
	loans.PUT("/:id", loanHandler.Update)

	loans.PATCH("/:id/review", middleware.RequireAnyRole(
		models.RoleSuperAdmin,
		models.RoleAdmin,
	), loanHandler.Review)
}
