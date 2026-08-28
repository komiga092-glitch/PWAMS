package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "0"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "camera=(), microphone=(), geolocation=()"},
		{"Cross-Origin-Opener-Policy", "same-origin"},
		{"Cross-Origin-Resource-Policy", "same-origin"},
	}

	for _, test := range tests {
		if w.Header().Get(test.header) != test.expected {
			t.Errorf("Header %s = %q, want %q", test.header, w.Header().Get(test.header), test.expected)
		}
	}
}

func TestCSRF_SafeMethodSkipsValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.CSRF())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET should skip CSRF, got %d", w.Code)
	}
}

func TestCSRF_MissingTokenRejects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.CSRF())
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST without CSRF token should return 403, got %d", w.Code)
	}
}
