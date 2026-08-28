package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const csrfCookieName = "pwams_csrf"

// EnsureCSRF issues a CSRF token cookie when one is absent and, for
// unsafe methods, validates the double-submit token. The cookie is
// intentionally NOT HttpOnly: browser JavaScript must read it to echo
// the value back in the X-CSRF-Token header (or as the _csrf form
// field). The session cookie itself remains HttpOnly.
func EnsureCSRF(secure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			if cookieValue(c) == "" {
				setCSRFToken(c, secure)
			}
			c.Next()
			return
		}

		cookieToken := cookieValue(c)
		if cookieToken == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "CSRF token missing",
			})
			return
		}

		headerToken := c.GetHeader("X-CSRF-Token")
		if headerToken == "" {
			headerToken = c.PostForm("_csrf")
		}

		if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "CSRF token mismatch",
			})
			return
		}

		c.Next()
	}
}

// CSRF returns validation-only middleware (kept for tests and callers
// that manage token issuance themselves).
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}

		cookieToken := cookieValue(c)
		if cookieToken == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "CSRF token missing",
			})
			return
		}

		headerToken := c.GetHeader("X-CSRF-Token")
		if headerToken == "" {
			headerToken = c.PostForm("_csrf")
		}

		if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "CSRF token mismatch",
			})
			return
		}

		c.Next()
	}
}

func cookieValue(c *gin.Context) string {
	value, err := c.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return value
}

// SetCSRFToken generates and sets a new CSRF token cookie.
func SetCSRFToken(c *gin.Context, secure bool) {
	setCSRFToken(c, secure)
}

func setCSRFToken(c *gin.Context, secure bool) {
	token := generateCSRFToken()

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		csrfCookieName,
		token,
		3600,
		"/",
		"",
		secure,
		false, // Not HttpOnly: must be readable by first-party JS.
	)

	c.Header("X-CSRF-Token", token)
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate CSRF token: " + err.Error())
	}
	// RawURLEncoding avoids '=' padding characters which can be
	// percent-encoded inconsistently across clients and break
	// double-submit comparison.
	return base64.RawURLEncoding.EncodeToString(b)
}

func isSafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
