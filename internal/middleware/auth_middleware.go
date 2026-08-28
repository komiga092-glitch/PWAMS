package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/services"
)

const sessionCookieName = "pwams_session"

type AuthMiddleware struct {
	sessionService *services.SessionService
	secureCookie   bool
}

func NewAuthMiddleware(
	sessionService *services.SessionService,
	secureCookie ...bool,
) *AuthMiddleware {
	sc := false
	if len(secureCookie) > 0 {
		sc = secureCookie[0]
	}
	return &AuthMiddleware{
		sessionService: sessionService,
		secureCookie:   sc,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Authenticated responses must never be cached by browsers
		// or shared caches (they contain user-specific data).
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
		c.Header("Pragma", "no-cache")

		rawToken, err := c.Cookie(sessionCookieName)
		if err != nil || rawToken == "" {
			abortUnauthenticated(c, "Authentication required")
			return
		}

		user, err := m.sessionService.ValidateSession(rawToken)
		if err != nil {
			abortUnauthenticated(c, "Session is invalid or expired")
			return
		}

		c.Set("current_user", user)
		c.Next()
	}
}

func abortUnauthenticated(c *gin.Context, message string) {
	if isBrowserRequest(c) {
		c.Redirect(http.StatusSeeOther, "/login")
		c.Abort()
		return
	}

	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"message": message,
	})
}

func isBrowserRequest(c *gin.Context) bool {
	acceptsHTML := strings.Contains(c.GetHeader("Accept"), "text/html")
	isNavigation := c.GetHeader("Sec-Fetch-Mode") == "navigate" &&
		c.GetHeader("Sec-Fetch-Dest") == "document"
	isPageRoute := c.Request.Method == http.MethodGet &&
		(c.Request.URL.Path == "/dashboard" ||
			c.Request.URL.Path == "/profile" ||
			strings.HasSuffix(c.Request.URL.Path, "/page") ||
			strings.HasSuffix(c.Request.URL.Path, "/view") ||
			strings.HasSuffix(c.Request.URL.Path, "/edit"))
	return acceptsHTML || isNavigation || isPageRoute || c.GetHeader("HX-Request") == "true"
}
