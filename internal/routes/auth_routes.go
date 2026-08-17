package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterAuthRoutes(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	dashboardHandler *handlers.DashboardHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	// =========================
	// Public authentication
	// =========================

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title": "PWAMS Login",
		})
	})
	router.GET("/forgot-password", func(c *gin.Context) {
	c.HTML(http.StatusOK, "forgot_password.html", gin.H{
		"title": "Forgot Password",
	})
})
	router.GET("/verify-reset-otp", func(c *gin.Context) {
		c.HTML(http.StatusOK, "verify_reset_otp.html", gin.H{
			"title": "Verify OTP",
		})
	})
	router.GET("/reset-password", func(c *gin.Context) {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"title": "Reset Password",
		})
	})

	router.POST(
		"/login",
		authHandler.Login,
	)

	router.POST(
		"/forgot-password",
		authHandler.ForgotPassword,
	)

	router.POST(
		"/verify-reset-otp",
		authHandler.VerifyResetOTP,
	)

	router.POST(
		"/reset-password",
		authHandler.ResetPassword,
	)

	// =========================
	// Protected routes
	// =========================

	protected := router.Group("/")
	protected.Use(
		authMiddleware.RequireAuth(),
	)

	protected.GET(
		"/dashboard",
		dashboardHandler.Page,
	)

	protected.POST(
		"/logout",
		authHandler.Logout,
	)

	// Admin-level test route.
	protected.GET(
		"/admin-test",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Admin-level access granted",
			})
		},
	)
}
