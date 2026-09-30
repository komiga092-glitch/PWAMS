package routes_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/routes"
	"github.com/komiga092-glitch/pwams/internal/services"
	"github.com/komiga092-glitch/pwams/internal/services/email"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

// ─────────────────────────────────────────────────────────────────────────────
// FLOW 8 (password reset) and FLOW 9 (account activation) blackbox tests.
//
// The whole chain is exercised through the real Gin handlers, the real CSRF
// middleware and the real database: no HTTP stubs and no direct service calls
// for the parts under test. Only the OTP read-back (which a human would get
// from their inbox) is done with GORM, because this environment has no SMTP
// server configured.
// ─────────────────────────────────────────────────────────────────────────────

var (
	authFlowTestDB     *gorm.DB
	authFlowTestCfg    *config.Config
	authFlowTestDBOnce sync.Once
	authFlowTestDBErr  error
)

func acquireAuthFlowTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	authFlowTestDBOnce.Do(func() {
		_ = godotenv.Load("../../.env")

		cfg, err := config.Load()
		if err != nil {
			authFlowTestDBErr = err
			return
		}
		authFlowTestCfg = cfg

		authFlowTestDB, authFlowTestDBErr = database.Connect(cfg)
		if authFlowTestDBErr != nil {
			return
		}
		if err := database.Migrate(authFlowTestDB); err != nil {
			authFlowTestDBErr = err
			return
		}
		if err := database.SeedDefaultRoles(authFlowTestDB); err != nil {
			authFlowTestDBErr = err
		}
	})

	if authFlowTestDBErr != nil {
		t.Skipf("integration test DB unavailable: %v", authFlowTestDBErr)
	}
	if authFlowTestDB == nil {
		t.Skip("integration test DB unavailable: connection is nil")
	}

	return authFlowTestDB
}

type authFlowApp struct {
	router     *gin.Engine
	db         *gorm.DB
	sessions   *services.SessionService
	clientAddr string
}

// asClient returns the same app bound to a different simulated client IP.
// Rate limits are per client, so each phase of a flow uses its own client,
// exactly like distinct users sitting behind distinct addresses.
func (a *authFlowApp) asClient(ip string) *authFlowApp {
	return &authFlowApp{
		router:     a.router,
		db:         a.db,
		sessions:   a.sessions,
		clientAddr: ip + ":54321",
	}
}

func (a *authFlowApp) remoteAddr() string {
	if a.clientAddr == "" {
		return "203.0.113.10:54321"
	}
	return a.clientAddr
}

func newAuthFlowApp(t *testing.T, db *gorm.DB) *authFlowApp {
	t.Helper()

	gin.SetMode(gin.TestMode)

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	emailService := email.NewEmailService(authFlowTestCfg)
	sessionService := services.NewSessionService(sessionRepo)
	authMiddleware := middleware.NewAuthMiddleware(sessionService)

	authHandler := handlers.NewAuthHandler(
		services.NewAuthService(userRepo),
		sessionService,
		services.NewPasswordResetService(
			repository.NewPasswordResetRepository(db),
			userRepo,
			emailService,
			sessionRepo,
		),
		services.NewAuditLogService(repository.NewAuditLogRepository(db)),
		false,
	)

	activationHandler := handlers.NewAccountActivationHandler(
		services.NewAccountActivationService(
			repository.NewAccountActivationRepository(db),
			userRepo,
			emailService,
		),
	)

	router := gin.New()
	router.Use(middleware.EnsureCSRF(false))
	router.Use(middleware.ResolveLocale(false))

	// dashboardHandler is only referenced (never called) by RegisterAuthRoutes;
	// a nil pointer method value is safe and the dashboard page is not part of
	// these flows.
	routes.RegisterAuthRoutes(router, authHandler, nil, authMiddleware)
	routes.RegisterAccountActivationRoutes(router, activationHandler)

	return &authFlowApp{router: router, db: db, sessions: sessionService}
}

func authFlowCSRF(t *testing.T, app *authFlowApp) (string, *http.Cookie) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /login: status=%d", rec.Code)
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "pwams_csrf" && cookie.Value != "" {
			return cookie.Value, &http.Cookie{Name: cookie.Name, Value: cookie.Value}
		}
	}

	t.Fatal("GET /login did not issue a CSRF cookie")
	return "", nil
}

// authFlowPostJSON posts JSON with the CSRF header + cookie, exactly as the
// browser-side fetch helper does.
func authFlowPostJSON(
	t *testing.T,
	app *authFlowApp,
	path string,
	csrf string,
	csrfCookie *http.Cookie,
	payload any,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.RemoteAddr = app.remoteAddr()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	if csrfCookie != nil {
		req.AddCookie(csrfCookie)
	}
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}

	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)
	return rec
}

func authFlowPostForm(
	t *testing.T,
	app *authFlowApp,
	path string,
	csrfCookie *http.Cookie,
	form url.Values,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.RemoteAddr = app.remoteAddr()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if csrfCookie != nil {
		req.AddCookie(csrfCookie)
	}

	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)
	return rec
}

// authFlowLogin performs a form login and returns the response.
func authFlowLogin(
	t *testing.T,
	app *authFlowApp,
	csrfCookie *http.Cookie,
	login string,
	password string,
) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{}
	form.Set("login", login)
	form.Set("password", password)
	if csrfCookie != nil {
		form.Set("_csrf", csrfCookie.Value)
	}

	return authFlowPostForm(t, app, "/login", csrfCookie, form)
}

// createAuthFlowProbeUser creates a throwaway user with a REAL bcrypt hash so
// the login handler can authenticate it. Cleanup removes exactly the rows this
// test created.
func createAuthFlowProbeUser(
	t *testing.T,
	db *gorm.DB,
	status string,
	password string,
) *models.User {
	t.Helper()

	var role models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&role).Error; err != nil {
		t.Fatalf("role %q not found: %v", models.RoleStaff, err)
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		t.Fatalf("hash probe password: %v", err)
	}

	stamp := time.Now().UnixNano()
	user := &models.User{
		Username:     "flow_probe_" + time.Unix(0, stamp).Format("150405.000000000"),
		Email:        "flow_probe_" + time.Unix(0, stamp).Format("150405.000000000") + "@example.test",
		PasswordHash: hash,
		RoleID:       role.ID,
		Status:       status,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create probe user: %v", err)
	}

	t.Cleanup(func() {
		db.Where("user_id = ?", user.ID).Delete(&models.Session{})
		db.Where("user_id = ?", user.ID).Delete(&models.PasswordResetToken{})
		db.Where("user_id = ?", user.ID).Delete(&models.AccountActivationToken{})
		db.Where("details = ?", user.Email).Delete(&models.AuditLog{})
		db.Unscoped().Delete(&models.User{}, "id = ?", user.ID)
	})

	return user
}

func authFlowSessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "pwams_session" && cookie.Value != "" {
			return cookie
		}
	}
	return nil
}

func authFlowLatestResetToken(t *testing.T, db *gorm.DB, userID any) *models.PasswordResetToken {
	t.Helper()

	var token models.PasswordResetToken
	if err := db.Where("user_id = ?", userID).Order("created_at desc").First(&token).Error; err != nil {
		t.Fatalf("password reset token not found: %v", err)
	}
	return &token
}

func authFlowLatestActivationToken(t *testing.T, db *gorm.DB, userID any) *models.AccountActivationToken {
	t.Helper()

	var token models.AccountActivationToken
	if err := db.Where("user_id = ?", userID).Order("created_at desc").First(&token).Error; err != nil {
		t.Fatalf("account activation token not found: %v", err)
	}
	return &token
}

func authFlowPasswordHash(t *testing.T, db *gorm.DB, userID any) string {
	t.Helper()

	var user models.User
	if err := db.Select("password_hash").Where("id = ?", userID).First(&user).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return user.PasswordHash
}

func authFlowUserStatus(t *testing.T, db *gorm.DB, userID any) string {
	t.Helper()

	var user models.User
	if err := db.Select("status").Where("id = ?", userID).First(&user).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return user.Status
}

func authFlowGet(t *testing.T, app *authFlowApp, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = app.remoteAddr()
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}

	rec := httptest.NewRecorder()
	app.router.ServeHTTP(rec, req)
	return rec
}

// ─────────────────────────────────────────────────────────────────────────────
// FLOW 8: forgot password -> OTP -> verify -> reset -> login with new password
// ─────────────────────────────────────────────────────────────────────────────

func TestPasswordResetFlowEndToEnd(t *testing.T) {
	db := acquireAuthFlowTestDB(t)
	app := newAuthFlowApp(t, db)

	const initialPassword = "InitialPass123"
	const newPassword = "RotatedPass456"

	user := createAuthFlowProbeUser(t, db, models.UserStatusActive, initialPassword)
	csrf, csrfCookie := authFlowCSRF(t, app)

	// Password-reset and OTP endpoints are rate limited per client (3 requests
	// per window). Each phase therefore acts as its own client, which is what
	// the limit is designed to model.
	validationClient := app.asClient("198.51.100.11")
	otpClient := app.asClient("198.51.100.12")
	resetClient := app.asClient("198.51.100.13")
	replayClient := app.asClient("198.51.100.14")

	// A live session that the reset must invalidate.
	preSession, _, err := app.sessions.CreateSession(user.ID)
	if err != nil {
		t.Fatalf("create pre-reset session: %v", err)
	}
	if _, err := app.sessions.ValidateSession(preSession); err != nil {
		t.Fatalf("pre-reset session should be valid: %v", err)
	}

	// CSRF protection must reject an unsafe request without the token.
	rec := authFlowPostJSON(t, app, "/forgot-password", "", nil, map[string]string{"email": user.Email})
	if rec.Code != http.StatusForbidden {
		t.Errorf("forgot-password without CSRF: expected 403, got %d", rec.Code)
	}

	// Malformed email is a validation failure.
	rec = authFlowPostJSON(t, validationClient, "/forgot-password", csrf, csrfCookie, map[string]string{"email": "not-an-email"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("forgot-password malformed email: expected 400, got %d", rec.Code)
	}

	// Unknown accounts must not be enumerable.
	rec = authFlowPostJSON(t, validationClient, "/forgot-password", csrf, csrfCookie, map[string]string{"email": "nobody@example.test"})
	if rec.Code != http.StatusOK {
		t.Errorf("forgot-password unknown account: expected 200 (anti-enumeration), got %d", rec.Code)
	}

	// Known account: the OTP row must exist. Delivery itself depends on SMTP,
	// which this environment does not configure.
	rec = authFlowPostJSON(t, otpClient, "/forgot-password", csrf, csrfCookie, map[string]string{"email": user.Email})
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Errorf("forgot-password known account: expected 200 or 500, got %d (body %.200s)", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusInternalServerError {
		t.Log("SMTP is not configured for this environment: OTP delivery returns 500 while the token row is still created")
	}

	token := authFlowLatestResetToken(t, db, user.ID)
	if len(token.OTP) != 6 {
		t.Fatalf("reset OTP must be 6 characters, got %q", token.OTP)
	}
	for _, r := range token.OTP {
		if r < '0' || r > '9' {
			t.Fatalf("reset OTP must be numeric, got %q", token.OTP)
		}
	}
	if !token.ExpiresAt.After(time.Now()) {
		t.Errorf("reset OTP expiry must be in the future, got %s", token.ExpiresAt)
	}
	if token.Verified || token.UsedAt != nil {
		t.Error("fresh reset token must be unverified and unused")
	}
	if strings.Contains(rec.Body.String(), token.OTP) {
		t.Error("password reset response must never expose the OTP")
	}

	// Wrong OTP.
	rec = authFlowPostJSON(t, otpClient, "/verify-reset-otp", csrf, csrfCookie,
		map[string]string{"email": user.Email, "otp": "000000"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("verify-reset-otp wrong OTP: expected 400, got %d", rec.Code)
	}
	if authFlowLatestResetToken(t, db, user.ID).Verified {
		t.Error("wrong OTP must not verify the token")
	}

	// Malformed OTP length.
	rec = authFlowPostJSON(t, otpClient, "/verify-reset-otp", csrf, csrfCookie,
		map[string]string{"email": user.Email, "otp": "123"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("verify-reset-otp short OTP: expected 400, got %d", rec.Code)
	}

	// Correct OTP.
	rec = authFlowPostJSON(t, otpClient, "/verify-reset-otp", csrf, csrfCookie,
		map[string]string{"email": user.Email, "otp": token.OTP})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify-reset-otp correct OTP: expected 200, got %d (body %.200s)", rec.Code, rec.Body.String())
	}
	if !authFlowLatestResetToken(t, db, user.ID).Verified {
		t.Error("correct OTP must mark the token verified")
	}

	hashBefore := authFlowPasswordHash(t, db, user.ID)

	// Mismatched confirmation.
	rec = authFlowPostJSON(t, resetClient, "/reset-password", csrf, csrfCookie, map[string]string{
		"email": user.Email, "otp": token.OTP,
		"new_password": newPassword, "confirm_password": "DifferentPass789",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("reset-password mismatch: expected 400, got %d", rec.Code)
	}
	if authFlowPasswordHash(t, db, user.ID) != hashBefore {
		t.Error("rejected reset must not change the stored password")
	}

	// Too short.
	rec = authFlowPostJSON(t, resetClient, "/reset-password", csrf, csrfCookie, map[string]string{
		"email": user.Email, "otp": token.OTP,
		"new_password": "short", "confirm_password": "short",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("reset-password too short: expected 400, got %d", rec.Code)
	}
	if authFlowPasswordHash(t, db, user.ID) != hashBefore {
		t.Error("rejected reset must not change the stored password")
	}

	// Valid reset.
	rec = authFlowPostJSON(t, resetClient, "/reset-password", csrf, csrfCookie, map[string]string{
		"email": user.Email, "otp": token.OTP,
		"new_password": newPassword, "confirm_password": newPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset-password valid: expected 200, got %d (body %.200s)", rec.Code, rec.Body.String())
	}

	hashAfter := authFlowPasswordHash(t, db, user.ID)
	if hashAfter == hashBefore {
		t.Error("password hash must change after a successful reset")
	}
	if strings.Contains(hashAfter, newPassword) {
		t.Error("password must never be stored in plaintext")
	}

	if consumed := authFlowLatestResetToken(t, db, user.ID); consumed.UsedAt == nil {
		t.Error("successful reset must consume the OTP (used_at set)")
	}

	if _, err := app.sessions.ValidateSession(preSession); err == nil {
		t.Error("password reset must revoke existing sessions")
	}

	// Consumed OTP cannot be replayed.
	rec = authFlowPostJSON(t, replayClient, "/reset-password", csrf, csrfCookie, map[string]string{
		"email": user.Email, "otp": token.OTP,
		"new_password": newPassword, "confirm_password": newPassword,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("consumed OTP reuse: expected 400, got %d", rec.Code)
	}

	// Old password no longer works. The login page is re-rendered with an error
	// on purpose (so the HTMX swap keeps the form visible) and no session is
	// issued.
	rec = authFlowLogin(t, app, csrfCookie, user.Username, initialPassword)
	if rec.Code != http.StatusOK {
		t.Errorf("login with old password: expected 200 login page, got %d", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "invalid") {
		t.Errorf("login with old password must show an invalid-credentials message, body %.200s", rec.Body.String())
	}
	if authFlowSessionCookie(rec) != nil {
		t.Error("failed login must not issue a session cookie")
	}

	// New password works.
	rec = authFlowLogin(t, app, csrfCookie, user.Username, newPassword)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login with new password: expected 303, got %d (body %.200s)", rec.Code, rec.Body.String())
	}
	sessionCookie := authFlowSessionCookie(rec)
	if sessionCookie == nil {
		t.Fatal("successful login must issue a pwams_session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}

	if me := authFlowGet(t, app, "/auth/me", sessionCookie); me.Code != http.StatusOK {
		t.Errorf("/auth/me with the new session: expected 200, got %d", me.Code)
	}

	// Audit trail.
	var auditCount int64
	if err := db.Model(&models.AuditLog{}).
		Where("action = ? AND details = ?", "PASSWORD_RESET", user.Email).
		Count(&auditCount).Error; err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount < 1 {
		t.Error("password reset must write a PASSWORD_RESET audit record")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// FLOW 9: disabled account -> activation OTP -> verify -> reactivate -> login
// ─────────────────────────────────────────────────────────────────────────────

func TestAccountActivationFlowEndToEnd(t *testing.T) {
	db := acquireAuthFlowTestDB(t)
	app := newAuthFlowApp(t, db)

	const password = "DisabledPass123"

	disabled := createAuthFlowProbeUser(t, db, models.UserStatusDisabled, password)
	active := createAuthFlowProbeUser(t, db, models.UserStatusActive, password)
	csrf, csrfCookie := authFlowCSRF(t, app)

	// A disabled account must not be able to authenticate.
	rec := authFlowLogin(t, app, csrfCookie, disabled.Username, password)
	if rec.Code != http.StatusOK {
		t.Errorf("disabled account login: expected 200 login page, got %d", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "disabled") {
		t.Errorf("disabled account login must show a disabled message, body %.200s", rec.Body.String())
	}
	if authFlowSessionCookie(rec) != nil {
		t.Error("disabled account must not receive a session cookie")
	}

	// Validation: malformed email.
	rec = authFlowPostJSON(t, app, "/request-account-activation", csrf, csrfCookie,
		map[string]string{"email": "not-an-email"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("activation request malformed email: expected 400, got %d", rec.Code)
	}

	// Validation: unknown account.
	rec = authFlowPostJSON(t, app, "/request-account-activation", csrf, csrfCookie,
		map[string]string{"email": "nobody@example.test"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("activation request unknown account: expected 400, got %d", rec.Code)
	}

	// An already active account has nothing to activate.
	rec = authFlowPostJSON(t, app, "/request-account-activation", csrf, csrfCookie,
		map[string]string{"email": active.Email})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("activation request for active account: expected 400, got %d", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "already active") {
		t.Errorf("activation request for active account: expected 'already active', body %.200s", rec.Body.String())
	}

	// CSRF is enforced on the public activation endpoints too.
	rec = authFlowPostJSON(t, app, "/request-account-activation", "", nil,
		map[string]string{"email": disabled.Email})
	if rec.Code != http.StatusForbidden {
		t.Errorf("activation request without CSRF: expected 403, got %d", rec.Code)
	}

	// Valid request for the disabled account: the OTP row must exist.
	rec = authFlowPostJSON(t, app, "/request-account-activation", csrf, csrfCookie,
		map[string]string{"email": disabled.Email})
	if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
		t.Errorf("activation request: expected 200 or 400 (SMTP), got %d (body %.200s)", rec.Code, rec.Body.String())
	}

	token := authFlowLatestActivationToken(t, db, disabled.ID)
	if len(token.OTP) != 6 {
		t.Fatalf("activation OTP must be 6 characters, got %q", token.OTP)
	}
	if strings.Contains(rec.Body.String(), token.OTP) {
		t.Error("activation response must never expose the OTP")
	}

	// Reactivating before verification must fail.
	rec = authFlowPostJSON(t, app, "/reactivate-account", csrf, csrfCookie,
		map[string]string{"email": disabled.Email, "otp": token.OTP})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("reactivate before verification: expected 400, got %d", rec.Code)
	}
	if authFlowUserStatus(t, db, disabled.ID) != models.UserStatusDisabled {
		t.Error("reactivate before verification must not change the account status")
	}

	// Wrong OTP.
	rec = authFlowPostJSON(t, app, "/verify-account-activation-otp", csrf, csrfCookie,
		map[string]string{"email": disabled.Email, "otp": "111111"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("verify activation wrong OTP: expected 400, got %d", rec.Code)
	}

	// Correct OTP.
	rec = authFlowPostJSON(t, app, "/verify-account-activation-otp", csrf, csrfCookie,
		map[string]string{"email": disabled.Email, "otp": token.OTP})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify activation correct OTP: expected 200, got %d (body %.200s)", rec.Code, rec.Body.String())
	}
	if !authFlowLatestActivationToken(t, db, disabled.ID).Verified {
		t.Error("correct activation OTP must mark the token verified")
	}

	// Reactivate.
	rec = authFlowPostJSON(t, app, "/reactivate-account", csrf, csrfCookie,
		map[string]string{"email": disabled.Email, "otp": token.OTP})
	if rec.Code != http.StatusOK {
		t.Fatalf("reactivate account: expected 200, got %d (body %.200s)", rec.Code, rec.Body.String())
	}
	if status := authFlowUserStatus(t, db, disabled.ID); status != models.UserStatusActive {
		t.Errorf("account status after reactivation: expected %q, got %q", models.UserStatusActive, status)
	}
	if consumed := authFlowLatestActivationToken(t, db, disabled.ID); consumed.UsedAt == nil {
		t.Error("successful reactivation must consume the OTP (used_at set)")
	}

	// The reactivated account can now log in.
	rec = authFlowLogin(t, app, csrfCookie, disabled.Username, password)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("reactivated account login: expected 303, got %d (body %.200s)", rec.Code, rec.Body.String())
	}
	if authFlowSessionCookie(rec) == nil {
		t.Error("reactivated account login must issue a session cookie")
	}

	// The consumed OTP cannot be replayed.
	rec = authFlowPostJSON(t, app, "/verify-account-activation-otp", csrf, csrfCookie,
		map[string]string{"email": disabled.Email, "otp": token.OTP})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("consumed activation OTP reuse: expected 400, got %d", rec.Code)
	}
}

// TestLoginRateLimitIsEnforced proves the login limiter is really wired to the
// route (5 attempts per client per window, then HTTP 429 + Retry-After).
func TestLoginRateLimitIsEnforced(t *testing.T) {
	db := acquireAuthFlowTestDB(t)
	app := newAuthFlowApp(t, db)

	client := app.asClient("198.51.100.77")
	_, csrfCookie := authFlowCSRF(t, client)

	for attempt := 1; attempt <= 5; attempt++ {
		rec := authFlowLogin(t, client, csrfCookie, "definitely_not_a_user", "WrongPass123")
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: expected 200 login page, got %d", attempt, rec.Code)
		}
		if authFlowSessionCookie(rec) != nil {
			t.Fatalf("attempt %d must not issue a session cookie", attempt)
		}
	}

	rec := authFlowLogin(t, client, csrfCookie, "definitely_not_a_user", "WrongPass123")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("attempt 6: expected 429, got %d (body %.200s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 response must include a Retry-After header")
	}
}
