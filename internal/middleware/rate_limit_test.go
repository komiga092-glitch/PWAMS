package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func TestRateLimitGeneric_AllowsNormalTraffic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RateLimitGeneric())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i, w.Code)
		}
	}
}

// TestRateLimitLogin_NotBypassedBySpoofedForwardedFor locks in the fix for the
// brute-force protection bypass: PWAMS must configure Gin with no trusted
// proxy unless TRUSTED_PROXIES is set, so a client supplied X-Forwarded-For
// header cannot be used to mint a fresh rate-limit bucket per request.
func TestRateLimitLogin_NotBypassedBySpoofedForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// No trusted proxy: cmd/server/main.go does the same through
	// router.SetTrustedProxies(cfg.TrustedProxies).
	if err := router.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil) failed: %v", err)
	}

	router.Use(middleware.RateLimitLogin())
	router.POST("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	throttled := false

	// The login limiter allows 5 attempts per 15 minutes. Every request below
	// rotates the spoofed header, which previously created a new bucket each
	// time and allowed unlimited attempts. The peer address stays constant, so
	// a single bucket is expected. A dedicated peer address keeps this test
	// independent from the other rate-limit tests in this package (the
	// limiter is process global).
	for i := 0; i < 12; i++ {
		req := httptest.NewRequest("POST", "/login", nil)
		req.RemoteAddr = "198.51.100.11:40000"
		req.Header.Set("X-Forwarded-For", "10.9.9."+strconv.Itoa(i))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			throttled = true
			break
		}
	}

	if !throttled {
		t.Fatal("login rate limit was bypassed by rotating X-Forwarded-For headers")
	}
}

// TestRateLimitLogin_TrustedProxyHeaderHonoured verifies that when a reverse
// proxy is explicitly trusted its X-Forwarded-For header still selects the
// rate-limit bucket, so deployments behind nginx keep per-client limits.
func TestRateLimitLogin_TrustedProxyHeaderHonoured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	if err := router.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		t.Fatalf("SetTrustedProxies failed: %v", err)
	}

	router.Use(middleware.RateLimitLogin())
	router.POST("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 5 attempts are allowed, so the 6th (index 5) is the first throttled.
	throttledAt := -1
	for i := 0; i < 12; i++ {
		req := httptest.NewRequest("POST", "/login", nil)
		req.RemoteAddr = "127.0.0.1:40001"
		req.Header.Set("X-Forwarded-For", "203.0.113.77")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			throttledAt = i
			break
		}
	}

	if throttledAt != 5 {
		t.Fatalf("expected the 6th attempt to be throttled, got index %d", throttledAt)
	}
}
