package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/web/templates/components"
)

func renderTempl(c *gin.Context, code int, fn func(ctx context.Context, w interface{ Write([]byte) (int, error) }) error) {
	c.Status(code)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := fn(context.Background(), c.Writer); err != nil {
		c.Error(err)
	}
}

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
		renderTempl(c, http.StatusOK, func(ctx context.Context, w interface{ Write([]byte) (int, error) }) error {
			return components.LoginPage("").Render(ctx, w)
		})
	})
	router.GET("/forgot-password", func(c *gin.Context) {
		c.HTML(http.StatusOK, "forgot_password.html", gin.H{})
	})
	router.GET("/verify-reset-otp", func(c *gin.Context) {
		c.HTML(http.StatusOK, "verify_reset_otp.html", gin.H{})
	})
	router.GET("/reset-password", func(c *gin.Context) {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{})
	})

	router.POST(
		"/login",
		middleware.RateLimitLogin(),
		authHandler.Login,
	)

	router.POST(
		"/forgot-password",
		middleware.RateLimitPasswordReset(),
		authHandler.ForgotPassword,
	)

	router.POST(
		"/verify-reset-otp",
		middleware.RateLimitOTP(),
		authHandler.VerifyResetOTP,
	)

	router.POST(
		"/reset-password",
		middleware.RateLimitPasswordReset(),
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
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
			models.RoleVolunteer,
			models.RoleDonor,
			models.RoleBeneficiary,
		),
		dashboardHandler.Page,
	)

	protected.POST(
		"/logout",
		authHandler.Logout,
	)

	protected.GET("/auth/me", func(c *gin.Context) {
		user, ok := c.Get("current_user")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Authentication required"})
			return
		}
		currentUser, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Invalid authentication context"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"username": currentUser.Username,
			"email":    currentUser.Email,
			"role":     currentUser.Role.Name,
		})
	})

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
