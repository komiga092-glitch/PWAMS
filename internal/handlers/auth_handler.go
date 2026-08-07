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
	authService    *services.AuthService
	sessionService *services.SessionService
	secureCookie   bool
}

func NewAuthHandler(
	authService *services.AuthService,
	sessionService *services.SessionService,
	secureCookie bool,
) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		sessionService: sessionService,
		secureCookie:   secureCookie,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request models.LoginRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return
	}

	user, err := h.authService.Login(
		request.Login,
		request.Password,
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	rawToken, expiresAt, err := h.sessionService.CreateSession(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrUnableToCreateLoginSession,
		})
		return
	}

	h.setSessionCookie(c, rawToken, expiresAt)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrLoginSuccessful,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role.Name,
			"status":   user.Status,
		},
	})
}

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

func (h *AuthHandler) setSessionCookie(
	c *gin.Context,
	rawToken string,
	expiresAt time.Time,
) {
	maxAge := int(time.Until(expiresAt).Seconds())

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawToken,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
