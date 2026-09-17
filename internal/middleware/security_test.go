package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestSecurityHeaders_CSPHardened pins the hardened Content-Security-Policy:
// 'unsafe-eval' must be gone (the htmx hx-on attributes that required it
// were removed), script-src stays 'self', and framing is fully denied.
func TestSecurityHeaders_CSPHardened(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header must be present")
	}

	if strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP must not contain 'unsafe-eval': %s", csp)
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP must pin script-src 'self': %s", csp)
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP must pin default-src 'self': %s", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP must pin frame-ancestors 'none': %s", csp)
	}
	if !strings.Contains(csp, "object-src 'none'") {
		t.Errorf("CSP must pin object-src 'none': %s", csp)
	}
	if !strings.Contains(csp, "base-uri 'self'") {
		t.Errorf("CSP must pin base-uri 'self': %s", csp)
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
