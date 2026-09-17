package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// RegisterSelfServiceRoutes registers the Beneficiary/Student self-service routes.
func RegisterSelfServiceRoutes(
	router *gin.Engine,
	selfServiceHandler *handlers.SelfServiceHandler,
	authMiddleware *middleware.AuthMiddleware,
	permissionService ...*services.PermissionService,
) {
	my := router.Group("/my")

	my.Use(authMiddleware.RequireAuth())
	my.Use(middleware.RequireAnyRole(
		models.RoleBeneficiary,
		models.RoleStudent,
	))

	// Load the effective permission set once per request so RequirePermission
	// and the permission-aware templates read from the request cache.
	if len(permissionService) > 0 && permissionService[0] != nil {
		permSvc := permissionService[0]
		my.Use(middleware.LoadPermissions(permSvc))

		// Self-service dashboard
		my.GET("/dashboard", selfServiceHandler.Dashboard)

		// My Aid
		my.GET("/aid",
			middleware.RequirePermission(permSvc, "aid.view_own"),
			selfServiceHandler.MyAid)

		// Request Aid
		my.GET("/aid/request",
			middleware.RequirePermission(permSvc, "aid.request"),
			selfServiceHandler.RequestAid)
		my.POST("/aid/request",
			middleware.RequirePermission(permSvc, "aid.request"),
			selfServiceHandler.RequestAidSubmit)

		// My Loans
		my.GET("/loans",
			middleware.RequirePermission(permSvc, "loan.view_own"),
			selfServiceHandler.MyLoans)

		// Apply for Loan
		my.GET("/loans/apply",
			middleware.RequirePermission(permSvc, "loan.apply"),
			selfServiceHandler.ApplyForLoan)
		my.POST("/loans/apply",
			middleware.RequirePermission(permSvc, "loan.apply"),
			selfServiceHandler.ApplyForLoanSubmit)

		// My Repayments
		my.GET("/repayments",
			middleware.RequirePermission(permSvc, "repayment.view_own"),
			selfServiceHandler.MyRepayments)

		// My Care
		my.GET("/care",
			middleware.RequirePermission(permSvc, "care.view_own"),
			selfServiceHandler.MyCare)
		// Notifications (self-service: own notifications only)
		my.GET("/notifications",
			middleware.RequirePermission(permSvc, "notification.view_own"),
			selfServiceHandler.MyNotifications)
		my.PATCH("/notifications/:id/read",
			middleware.RequirePermission(permSvc, "notification.view_own"),
			selfServiceHandler.MarkNotificationRead)
		return
	}

	// Fallback (no permission service configured): session + role gating
	// still applies above; fail-closed permission checks are unavailable.

	// Self-service dashboard
	my.GET("/dashboard", selfServiceHandler.Dashboard)

	// My Aid
	my.GET("/aid", selfServiceHandler.MyAid)

	// Request Aid
	my.GET("/aid/request", selfServiceHandler.RequestAid)
	my.POST("/aid/request", selfServiceHandler.RequestAidSubmit)

	// My Loans
	my.GET("/loans", selfServiceHandler.MyLoans)

	// Apply for Loan
	my.GET("/loans/apply", selfServiceHandler.ApplyForLoan)
	my.POST("/loans/apply", selfServiceHandler.ApplyForLoanSubmit)

	// My Repayments
	my.GET("/repayments", selfServiceHandler.MyRepayments)

	// My Care
	my.GET("/care", selfServiceHandler.MyCare)
	// Notifications (self-service: own notifications only)
	my.GET("/notifications", selfServiceHandler.MyNotifications)
	my.PATCH("/notifications/:id/read", selfServiceHandler.MarkNotificationRead)
}
