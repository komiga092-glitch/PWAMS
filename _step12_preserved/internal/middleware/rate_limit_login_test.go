package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/services"
)

// resetLoginLimiters empties the package-level login limiters between tests
// so cases never observe each other's recorded attempts.
func resetLoginLimiters(t *testing.T) {
	t.Helper()
	for _, rl := range []*rateLimiter{
		loginIPRequestLimiter,
		loginIPFailureLimiter,
		loginIdentifierFailureLimiter,
	} {
		rl.mu.Lock()
		rl.attempts = make(map[string][]time.Time)
		rl.mu.Unlock()
	}
}

// loginRouter builds a router with the real login rate-limit middleware in
// front of a stub handler that answers with the given status code, mimicking
// AuthHandler.Login (401 = invalid credentials, 303 = successful login that
// would redirect to the dashboard).
func loginRouter(t *testing.T, statusCode int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/login", RateLimitLogin(), func(c *gin.Context) {
		// Prove the middleware did not consume the request body.
		login := c.PostForm("login")
		c.String(statusCode, "identity=%s", login)
	})
	return router
}

// postLogin issues a form-encoded POST /login from the given client IP.
func postLogin(router *gin.Engine, ip, login string) *httptest.ResponseRecorder {
	body := strings.NewReader(
		"login=" + strings.ReplaceAll(login, " ", "+") + "&password=secret",
	)
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = ip + ":12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestLoginRateLimit_SuccessfulLoginsAreNotBlocked proves the core bug fix:
// many successful logins from the same IP and identifier never return 429.
// Under the previous limiter the 6th login within 15 minutes was blocked.
func TestLoginRateLimit_SuccessfulLoginsAreNotBlocked(t *testing.T) {
	resetLoginLimiters(t)
	router := loginRouter(t, http.StatusSeeOther)

	for i := 1; i <= 30; i++ {
		w := postLogin(router, "203.0.113.10", "person@example.com")
		if w.Code != http.StatusSeeOther {
			t.Fatalf("successful login %d: status = %d, want %d (body: %s)",
				i, w.Code, http.StatusSeeOther, w.Body.String())
		}
	}
}

// TestLoginRateLimit_MultipleSuccessfulLoginsDifferentUsersIsNotBlocked
// proves an office NAT with several legitimate users logging in is never
// throttled (the historical 429 root cause).
func TestLoginRateLimit_MultipleSuccessfulLoginsDifferentUsersIsNotBlocked(t *testing.T) {
	resetLoginLimiters(t)
	router := loginRouter(t, http.StatusSeeOther)

	for i := 0; i < 25; i++ {
		w := postLogin(router, "198.51.100.7", fmt.Sprintf("user%02d@example.com", i))
		if w.Code != http.StatusSeeOther {
			t.Fatalf("successful login %d: status = %d, want %d", i+1, w.Code, http.StatusSeeOther)
		}
	}
}

// TestLoginRateLimit_FailedAttemptsTripIdentifierLimit proves the
// brute-force shield: three failed attempts for one identifier are followed
// by a 429 with Retry-After (mirroring the AuthService account lock).
func TestLoginRateLimit_FailedAttemptsTripIdentifierLimit(t *testing.T) {
	resetLoginLimiters(t)
	router := loginRouter(t, http.StatusUnauthorized)

	for i := 1; i <= 3; i++ {
		w := postLogin(router, "203.0.113.20", "victim@example.com")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("failed attempt %d: status = %d, want %d", i, w.Code, http.StatusUnauthorized)
		}
	}

	w := postLogin(router, "203.0.113.20", "victim@example.com")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt after threshold: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	retryAfter := w.Header().Get("Retry-After")
	seconds, err := strconv.Atoi(retryAfter)
	if err != nil || seconds < 1 {
		t.Fatalf("Retry-After header = %q, want a positive integer", retryAfter)
	}
	// The remaining window can never exceed the 30-minute lockout duration.
	if seconds > int(services.LockoutDuration/time.Second) {
		t.Fatalf("Retry-After = %d, want <= %d", seconds, int(services.LockoutDuration/time.Second))
	}
}

// TestLoginRateLimit_SuccessfulLoginClearsIdentifierFailures proves the
// coordination with the AuthService counter: a user who mistypes twice and
// then signs in successfully is forgiven and may fail again later without
// being blocked.
func TestLoginRateLimit_SuccessfulLoginClearsIdentifierFailures(t *testing.T) {
	resetLoginLimiters(t)
	failRouter := loginRouter(t, http.StatusUnauthorized)
	successRouter := loginRouter(t, http.StatusSeeOther)

	// Two typos...
	for i := 0; i < 2; i++ {
		if w := postLogin(failRouter, "203.0.113.30", "worker@example.com"); w.Code != http.StatusUnauthorized {
			t.Fatalf("typo %d: status = %d, want 401", i+1, w.Code)
		}
	}
	// ...then a successful login...
	if w := postLogin(successRouter, "203.0.113.30", "worker@example.com"); w.Code != http.StatusSeeOther {
		t.Fatalf("successful login: status = %d, want 303", w.Code)
	}
	// ...then two more typos must still be allowed (counter was reset)...
	for i := 0; i < 2; i++ {
		if w := postLogin(failRouter, "203.0.113.30", "worker@example.com"); w.Code != http.StatusUnauthorized {
			t.Fatalf("post-success typo %d: status = %d, want 401", i+1, w.Code)
		}
	}
	// ...and the third failure after the success finally trips the limiter.
	if w := postLogin(failRouter, "203.0.113.30", "worker@example.com"); w.Code != http.StatusUnauthorized {
		t.Fatalf("third failure after success: status = %d, want 401", w.Code)
	}
	if w := postLogin(failRouter, "203.0.113.30", "worker@example.com"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt over threshold: status = %d, want 429", w.Code)
	}
}

// TestLoginRateLimit_IdentifierNormalization proves case/whitespace variants
// of the same login share one failed-attempt budget.
func TestLoginRateLimit_IdentifierNormalization(t *testing.T) {
	resetLoginLimiters(t)
	router := loginRouter(t, http.StatusUnauthorized)

	if w := postLogin(router, "203.0.113.40", "Admin@Example.COM"); w.Code != http.StatusUnauthorized {
		t.Fatalf("variant 1: status = %d, want 401", w.Code)
	}
	if w := postLogin(router, "203.0.113.40", "  admin@example.com "); w.Code != http.StatusUnauthorized {
		t.Fatalf("variant 2: status = %d, want 401", w.Code)
	}
	if w := postLogin(router, "203.0.113.40", "ADMIN@example.com"); w.Code != http.StatusUnauthorized {
		t.Fatalf("variant 3: status = %d, want 401", w.Code)
	}
	if w := postLogin(router, "203.0.113.40", "admin@example.com"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("same identifier after 3 variants: status = %d, want 429", w.Code)
	}
}

// TestLoginRateLimit_DifferentIdentifiersShareIPFailureBudget proves the
// IP-level distributed-username protection: failures for many identifiers
// from one IP exhaust a shared per-IP budget.
func TestLoginRateLimit_DifferentIdentifiersShareIPFailureBudget(t *testing.T) {
	resetLoginLimiters(t)
	router := loginRouter(t, http.StatusUnauthorized)

	const ipFailureLimit = 20
	for i := 0; i < ipFailureLimit; i++ {
		w := postLogin(router, "203.0.113.50", fmt.Sprintf("spray%02d@example.com", i))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("distributed failure %d: status = %d, want 401", i+1, w.Code)
		}
	}

	// The next failure — for yet another identifier — is IP-blocked.
	w := postLogin(router, "203.0.113.50", "one-more@example.com")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt after IP failure budget: status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 response must include Retry-After")
	}
}

// TestLoginRateLimit_OtherUsersOnSameIPUnaffectedByIdentifierLock proves the
// identifier throttle does not punish unrelated users behind the same NAT.
func TestLoginRateLimit_OtherUsersOnSameIPUnaffectedByIdentifierLock(t *testing.T) {
	resetLoginLimiters(t)
	failRouter := loginRouter(t, http.StatusUnauthorized)
	successRouter := loginRouter(t, http.StatusSeeOther)

	for i := 0; i < 3; i++ {
		if w := postLogin(failRouter, "203.0.113.60", "bruteforced@example.com"); w.Code != http.StatusUnauthorized {
			t.Fatalf("failed attempt %d: status = %d, want 401", i+1, w.Code)
		}
	}
	// The brute-forced identifier is now 429-throttled...
	if w := postLogin(failRouter, "203.0.113.60", "bruteforced@example.com"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("bruteforced identifier: status = %d, want 429", w.Code)
	}
	// ...but a different legitimate user from the same IP logs in fine.
	if w := postLogin(successRouter, "203.0.113.60", "colleague@example.com"); w.Code != http.StatusSeeOther {
		t.Fatalf("unaffected colleague: status = %d, want 303", w.Code)
	}
}

// TestLoginRateLimit_RequestBudgetGuardsFloods proves Layer 1: a flood of
// requests from one IP is throttled even when every attempt would succeed.
func TestLoginRateLimit_RequestBudgetGuardsFloods(t *testing.T) {
	resetLoginLimiters(t)
	router := loginRouter(t, http.StatusSeeOther)

	const requestLimit = 60
	for i := 0; i < requestLimit; i++ {
		if w := postLogin(router, "203.0.113.70", fmt.Sprintf("flood%02d@example.com", i)); w.Code != http.StatusSeeOther {
			t.Fatalf("request %d: status = %d, want 303", i+1, w.Code)
		}
	}

	w := postLogin(router, "203.0.113.70", "flood-overflow@example.com")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request over budget: status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 response must include Retry-After")
	}
}

// TestLoginRateLimit_BodyPreservedForHandler proves the middleware restores
// the request body after reading the identifier, so AuthHandler.Login still
// binds the credentials.
func TestLoginRateLimit_BodyPreservedForHandler(t *testing.T) {
	resetLoginLimiters(t)
	router := loginRouter(t, http.StatusOK)

	w := postLogin(router, "203.0.113.80", "form-user@example.com")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "identity=form-user@example.com") {
		t.Fatalf("handler did not receive form data, body = %q", w.Body.String())
	}
}

// TestLoginRateLimit_GETRequestsPassThrough proves reads are not throttled.
func TestLoginRateLimit_GETRequestsPassThrough(t *testing.T) {
	resetLoginLimiters(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/login", RateLimitLogin(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

// TestLoginRateLimit_JSONLoginIdentifier proves JSON logins are tracked the
// same way as form logins.
func TestLoginRateLimit_JSONLoginIdentifier(t *testing.T) {
	resetLoginLimiters(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/login", RateLimitLogin(), func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})

	doPost := func(login string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/login",
			strings.NewReader(`{"login":"`+login+`","password":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.90:5"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	for i := 0; i < 3; i++ {
		if w := doPost("json.user@example.com"); w.Code != http.StatusUnauthorized {
			t.Fatalf("JSON failure %d: status = %d, want 401", i+1, w.Code)
		}
	}
	if w := doPost("json.user@example.com"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("JSON attempt over threshold: status = %d, want 429", w.Code)
	}
}
