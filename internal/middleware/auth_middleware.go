package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/services"
)

const sessionCookieName = "pwams_session"

type AuthMiddleware struct {
	sessionService *services.SessionService
}

func NewAuthMiddleware(
	sessionService *services.SessionService,
) *AuthMiddleware {
	return &AuthMiddleware{
		sessionService: sessionService,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken, err := c.Cookie(sessionCookieName)
		if err != nil || rawToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required",
			})
			return
		}

		user, err := m.sessionService.ValidateSession(rawToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Session is invalid or expired",
			})
			return
		}

		c.Set("current_user", user)
		c.Next()
	}
}
