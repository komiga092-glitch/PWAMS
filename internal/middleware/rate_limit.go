package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func (rl *rateLimiter) startCleanupLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.prune()
		}
	}()
}

// prune removes stale entries (and empty keys) so the attempts map
// cannot grow without bound under IP spoofing or heavy traffic.
func (rl *rateLimiter) prune() {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for key, attempts := range rl.attempts {
		valid := attempts[:0]
		cutoff := now.Add(-rl.window)
		for _, t := range attempts {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.attempts, key)
		} else {
			rl.attempts[key] = valid
		}
	}
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	rl.startCleanupLoop()
	return rl
}

func (rl *rateLimiter) isAllowed(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	attempts := rl.attempts[key]
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	rl.attempts[key] = valid
	return len(valid) < rl.limit
}

func (rl *rateLimiter) record(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.attempts[key] = append(rl.attempts[key], time.Now())
}

var (
	loginLimiter   = newRateLimiter(5, 15*time.Minute)
	otpLimiter     = newRateLimiter(3, 10*time.Minute)
	resetLimiter   = newRateLimiter(3, 15*time.Minute)
	genericLimiter = newRateLimiter(30, 1*time.Minute)
)

// RateLimitLogin limits login attempts to 5 per 15 minutes per IP.
func RateLimitLogin() gin.HandlerFunc {
	return rateLimitMiddleware(loginLimiter)
}

// RateLimitOTP limits OTP send attempts to 3 per 10 minutes per IP.
func RateLimitOTP() gin.HandlerFunc {
	return rateLimitMiddleware(otpLimiter)
}

// RateLimitPasswordReset limits password reset to 3 per 15 minutes per IP.
func RateLimitPasswordReset() gin.HandlerFunc {
	return rateLimitMiddleware(resetLimiter)
}

// RateLimitGeneric limits to 30 requests per minute per IP.
func RateLimitGeneric() gin.HandlerFunc {
	return rateLimitMiddleware(genericLimiter)
}

func rateLimitMiddleware(rl *rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Reads (GET/HEAD/OPTIONS) are not throttled by the generic
		// limiter. The offline PWA polls /auth/me and loads many
		// static assets, which would otherwise exhaust the budget and
		// block legitimate page navigation.
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		key := c.ClientIP()

		if !rl.isAllowed(key) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "Too many requests. Please try again later.",
			})
			return
		}

		rl.record(key)
		c.Next()
	}
}
