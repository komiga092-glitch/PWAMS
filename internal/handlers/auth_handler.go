package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

const sessionCookieName = "pwams_session"

type AuthHandler struct {
	authService          *services.AuthService
	sessionService       *services.SessionService
	passwordResetService *services.PasswordResetService
	secureCookie         bool
}

func NewAuthHandler(
	authService *services.AuthService,
	sessionService *services.SessionService,
	passwordResetService *services.PasswordResetService,
	secureCookie bool,
) *AuthHandler {
	return &AuthHandler{
		authService:          authService,
		sessionService:       sessionService,
		passwordResetService: passwordResetService,
		secureCookie:         secureCookie,
	}
}

// Login authenticates the user, creates a session,
// stores the session token in an HttpOnly cookie,
// and redirects the user to the dashboard.
//
// @Summary User login
// @Description Authenticate a user and create a session
// @Tags Authentication
// @Accept application/x-www-form-urlencoded
// @Produce text/html
// @Param login formData string true "Email or username"
// @Param password formData string true "Password"
// @Success 303
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var request models.LoginRequest

	if err := c.ShouldBind(&request); err != nil {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"title": "PWAMS Login",
			"error": "Email / username and password are required",
		})
		return
	}

	user, err := h.authService.Login(
		request.Login,
		request.Password,
	)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "PWAMS Login",
			"error": err.Error(),
		})
		return
	}

	rawToken, expiresAt, err := h.sessionService.CreateSession(user.ID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"title": "PWAMS Login",
			"error": constants.ErrUnableToCreateLoginSession,
		})
		return
	}

	h.setSessionCookie(c, rawToken, expiresAt)

	c.Redirect(http.StatusSeeOther, "/dashboard")
}

// ForgotPassword starts the password reset process.
//
// @Summary Send password reset OTP
// @Description Sends a 6-digit password reset OTP to the user's registered email.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body models.ForgotPasswordRequest true "Forgot password request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var request models.ForgotPasswordRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrEmailRequired,
		})
		return
	}

	if err := h.passwordResetService.ForgotPassword(request.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrPasswordResetOTPSent,
	})
}

// VerifyResetOTP verifies the password reset OTP.
//
// @Summary Verify password reset OTP
// @Description Verifies the 6-digit OTP sent to the user's email.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body models.VerifyResetOTPRequest true "OTP verification request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /verify-reset-otp [post]
func (h *AuthHandler) VerifyResetOTP(c *gin.Context) {
	var request models.VerifyResetOTPRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "email and valid 6-digit OTP are required",
		})
		return
	}

	if err := h.passwordResetService.VerifyResetOTP(
		request.Email,
		request.OTP,
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP verified successfully",
	})
}

// ResetPassword changes the user's password after OTP verification.
//
// @Summary Reset password
// @Description Changes the user's password after successful OTP verification.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body models.ResetPasswordRequest true "Reset password request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var request models.ResetPasswordRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Email, OTP, new password and confirm password are required",
		})
		return
	}

	if request.NewPassword != request.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "New password and confirm password do not match",
		})
		return
	}

	if err := h.passwordResetService.ResetPassword(
		request.Email,
		request.OTP,
		request.NewPassword,
		request.ConfirmPassword,
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password reset successfully",
	})
}

// Logout revokes the current session and removes the session cookie.
//
// @Summary Logout
// @Description Revoke the current authenticated session.
// @Tags Authentication
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	rawToken, err := c.Cookie(sessionCookieName)

	if err == nil && rawToken != "" {
		if err := h.sessionService.RevokeSession(rawToken); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToEndSession,
			})
			return
		}
	}

	h.clearSessionCookie(c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrLogoutSuccessful,
	})
}

// Dashboard returns the authenticated user's information.
func (h *AuthHandler) Dashboard(c *gin.Context) {
	value, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return
	}

	user, ok := value.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrDashboardStatsRetrievedSuccessfully,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role.Name,
		},
	})
}

// setSessionCookie stores the session token in a secure HttpOnly cookie.
func (h *AuthHandler) setSessionCookie(
	c *gin.Context,
	rawToken string,
	expiresAt time.Time,
) {
	maxAge := int(time.Until(expiresAt).Seconds())

	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie removes the session cookie.
func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}
