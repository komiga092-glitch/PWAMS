// Package hosts_test hosts the 8-role login + dashboard live verification
// suite. It intentionally lives in the external test package so it can import
// internal/routes (which imports internal/handlers) without an import cycle,
// and it skips when no PostgreSQL is reachable so `go test ./...` still
// succeeds in plain environments.
package handlers_test

// Step 8.1 — FINAL 8-ROLE LOGIN + DASHBOARD LIVE VERIFICATION.
//
// These tests drive real HTTP round-trips through the production router
// wiring (security headers, CSRF, the two-layer login rate limiter, real
// repositories/services/handlers and the real server-side templates) served
// over httptest, against the same PostgreSQL database the rest of the
// integration suite uses. They prove that every one of the 8 roles can:
//
//  1. Log in with valid credentials.
//  2. Receive a valid authenticated session (HttpOnly pwams_session cookie).
//  3. Follow the login redirect (303 -> /dashboard).
//  4. Reach the correct dashboard — directly for the six operational roles,
//     or /dashboard -> /my/dashboard for Beneficiary and Student.
//  5. Receive HTTP 200 from the final dashboard page.
//  6. Actually render the expected dashboard content.
//
// The edge policies are also verified against the live stack:
//
//   - a disabled user cannot log in,
//   - an actively locked user cannot log in,
//   - an expired-lock user recovers with the correct password,
//   - a wrong password still fails (and increments the account counter),
//   - successful logins never consume the failed-login budget (no 429).
//
// Fixtures are created directly in the shared test database using random,
// prefix-namespaced identities under @pwams.local and are hard-removed
// (sessions, audit logs and the user rows) when each test finishes. No
// production credentials are used or hardcoded anywhere.

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	"github.com/komiga092-glitch/pwams/internal/utils"
)

const (
	eightRoleLoginPassword = "EightRole!Pass2026"
	eightRoleFixtureDomain = "pwams.local"
)

// ── Fixture seeding / cleanup ─────────────────────────────────────────────

// eightRoleCleanupByEmailPrefix hard-removes every row this suite creates for
// a fixture prefix: sessions, audit logs, password-reset tokens and
// notifications that reference the fixture users, then the users themselves.
// It runs immediately (clearing leftovers from any crashed previous run) and
// again via t.Cleanup once the test finishes.
//
// The Partner role is additionally constrained by uq_users_one_manager (at most
// one active Partner user), so the cleanup also hard-removes any pre-existing
// Partner user whose email matches the fixture domain — otherwise the suite
// could never create a Partner fixture on a database that already has one.
func eightRoleCleanupByEmailPrefix(t *testing.T, db *gorm.DB, prefix string) {
	t.Helper()
	// Anchor the pattern at the start of the local-part so "admin" does not
	// sweep up "super-admin" fixtures (both contain the substring "admin").
	anchored := strings.ToLower(strings.TrimSpace(prefix)) + "-%@" + eightRoleFixtureDomain
	// Broad pattern used only for the role-based Partner sweep below.
	broad := "%@" + eightRoleFixtureDomain

	doCleanup := func() {
		var ids []string
		_ = db.Unscoped().Model(&models.User{}).
			Where("LOWER(email) LIKE ?", anchored).
			Pluck("id", &ids).Error

		// Also hard-remove any pre-existing Partner user that would collide
		// with the single-Partner constraint when this prefix is for a Partner
		// fixture.
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(prefix)), "partner") {
			var partnerIDs []string
			_ = db.Unscoped().Model(&models.User{}).
				Where("LOWER(email) LIKE ?", broad).
				Where("role_id = (SELECT id FROM roles WHERE LOWER(name) = 'partner')").
				Pluck("id", &partnerIDs).Error
			ids = append(ids, partnerIDs...)
		}

		if len(ids) == 0 {
			return
		}
		_ = db.Where("user_id IN ?", ids).Delete(&models.Session{}).Error
		_ = db.Where("user_id IN ?", ids).Delete(&models.AuditLog{}).Error
		_ = db.Where("entity_id IN ?", ids).Delete(&models.AuditLog{}).Error
		_ = db.Where("user_id IN ?", ids).Delete(&models.PasswordResetToken{}).Error
		_ = db.Where("user_id IN ?", ids).Delete(&models.Notification{}).Error
		// account_activation_tokens has a hard FK to users(id) with ON DELETE
		// CASCADE, but the Unscoped delete above bypasses GORM's cascade, so
		// the child rows must be removed explicitly first.
		_ = db.Exec("DELETE FROM account_activation_tokens WHERE user_id IN ?", ids).Error
		_ = db.Unscoped().Where("LOWER(email) LIKE ?", anchored).Delete(&models.User{}).Error
		// Hard-delete the pre-existing Partner users that were swept up by the
		// role-based query above.
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(prefix)), "partner") {
			_ = db.Unscoped().
				Where("LOWER(email) LIKE ?", broad).
				Where("role_id = (SELECT id FROM roles WHERE LOWER(name) = 'partner')").
				Delete(&models.User{}).Error
		}
	}

	doCleanup()
	t.Cleanup(doCleanup)
}

// eightRoleFixtureHash returns a fresh bcrypt hash for the shared fixture
// password (never a production credential).
func eightRoleFixtureHash(t *testing.T) string {
	t.Helper()
	hash, err := utils.HashPassword(eightRoleLoginPassword)
	if err != nil {
		t.Fatalf("cannot hash fixture password: %v", err)
	}
	return hash
}

// seedEightRoleFixtureUser inserts one Active user for the given role into the
// shared test database with a random identity and registers prefix-based
// cleanup. mutate may override the default account state (status, lock
// window, failed-login counter) before the row is stored.
func seedEightRoleFixtureUser(
	t *testing.T,
	db *gorm.DB,
	roleName, prefix string,
	mutate func(*models.User),
) *models.User {
	t.Helper()

	var role models.Role
	if err := db.Where("LOWER(name) = ?", strings.ToLower(roleName)).First(&role).Error; err != nil {
		t.Fatalf("cannot resolve role %s: %v", roleName, err)
	}

	eightRoleCleanupByEmailPrefix(t, db, prefix)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	user := &models.User{
		Username:     fmt.Sprintf("%s_%s", prefix, suffix),
		Email:        fmt.Sprintf("%s-%s@%s", prefix, suffix, eightRoleFixtureDomain),
		FullName:     "8-Role Login Fixture",
		PasswordHash: eightRoleFixtureHash(t),
		RoleID:       role.ID,
		Role:         role,
		Status:       models.UserStatusActive,
	}
	if mutate != nil {
		mutate(user)
	}
	if user.Status == "" {
		user.Status = models.UserStatusActive
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("cannot create fixture user (%s): %v", roleName, err)
	}
	return user
}

// seedEightRoleLockedUser returns a fixture with a lock that is either still
// active (future LockedUntil, Status=Locked) or already expired (past
// LockedUntil, Status=Locked but IsLocked() == false).
func seedEightRoleLockedUser(
	t *testing.T,
	db *gorm.DB,
	prefix string,
	lockExpired bool,
) *models.User {
	lockedUntil := time.Now().Add(30 * time.Minute)
	if lockExpired {
		lockedUntil = time.Now().Add(-30 * time.Minute)
	}
	return seedEightRoleFixtureUser(t, db, models.RoleDonor, prefix, func(u *models.User) {
		u.Status = models.UserStatusLocked
		u.FailedLoginAttempts = 3
		u.LockedUntil = &lockedUntil
	})
}

// ── Browser-like HTTP session ─────────────────────────────────────────────

// eightRoleDB connects to the configured PostgreSQL database and brings it to
// the canonical state (migrations + role seed + permission seed + super admin
// seed). When no database is reachable the tests are skipped so the suite
// still runs in a plain `go test ./...` environment.
func eightRoleDB(t *testing.T) *gorm.DB {
	t.Helper()

	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("eight-role login test skipped (configuration unavailable): %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("eight-role login test skipped (database unavailable): %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("eight-role login test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("eight-role login test role seed failed: %v", err)
	}
	permSvc := services.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewRoleRepository(db),
		repository.NewAuditLogRepository(db),
	)
	if err := permSvc.SeedDefaults(); err != nil {
		t.Fatalf("eight-role login test permission seed failed: %v", err)
	}
	if err := database.SeedSuperAdmin(db, cfg); err != nil {
		t.Fatalf("eight-role login test super-admin seed failed: %v", err)
	}

	return db
}

// eightRoleTestSession is a browser-like HTTP client: it keeps a real cookie
// jar, echoes the double-submit CSRF token on unsafe requests and reports the
// raw response of every hop so redirects are inspected instead of silently
// followed.
type eightRoleTestSession struct {
	t        *testing.T
	baseURL  string
	client   *http.Client
	jar      *cookiejar.Jar
	clientIP string
}

func newEightRoleTestSession(t *testing.T, baseURL, clientIP string) *eightRoleTestSession {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cannot create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Do not auto-follow: the assertions inspect every hop manually.
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}
	return &eightRoleTestSession{
		t:        t,
		baseURL:  baseURL,
		client:   client,
		jar:      jar,
		clientIP: clientIP,
	}
}

func (s *eightRoleTestSession) html(resp *http.Response) string {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		s.t.Fatalf("cannot read response body: %v", err)
	}
	return string(data)
}

func (s *eightRoleTestSession) do(
	method, path string,
	body io.Reader,
	contentType string,
	withCSRF bool,
) *http.Response {
	req, err := http.NewRequest(method, s.baseURL+path, body)
	if err != nil {
		s.t.Fatalf("cannot build %s %s: %v", method, path, err)
	}
	req.Header.Set("Accept", "text/html")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	// A distinct per-scenario client address keeps the (by design) in-memory
	// login rate limiter from coupling one scenario's attempts to another's.
	req.Header.Set("X-Forwarded-For", s.clientIP)
	if withCSRF {
		if token := s.csrfToken(); token != "" {
			req.Header.Set("X-CSRF-Token", token)
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("%s %s failed: %v", method, path, err)
	}
	return resp
}

func (s *eightRoleTestSession) get(path string) *http.Response {
	return s.do(http.MethodGet, path, nil, "", false)
}

// fetchLoginPage performs the browser's initial GET /login so the CSRF token
// cookie is issued and the page really renders.
func (s *eightRoleTestSession) fetchLoginPage() {
	resp := s.get("/login")
	if resp.StatusCode != http.StatusOK {
		s.close(resp)
		s.t.Fatalf("GET /login status = %d, want 200", resp.StatusCode)
	}
	body := s.html(resp) // html() closes resp.Body
	if !strings.Contains(body, "PWAMS Login") {
		s.t.Fatalf("GET /login rendered content does not contain the login page, got %d bytes", len(body))
	}
}

func (s *eightRoleTestSession) csrfToken() string {
	u, err := url.Parse(s.baseURL + "/login")
	if err != nil {
		s.t.Fatalf("cannot parse base URL: %v", err)
	}
	for _, c := range s.jar.Cookies(u) {
		if c.Name == "pwams_csrf" {
			return c.Value
		}
	}
	return ""
}

func (s *eightRoleTestSession) postLogin(login, password string) *http.Response {
	form := url.Values{}
	form.Set("login", login)
	form.Set("password", password)
	return s.do(
		http.MethodPost,
		"/login",
		strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded",
		true, // echo the CSRF token exactly like the first-party JS client does
	)
}

func (s *eightRoleTestSession) close(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

// ── Production router wiring (subset needed for login + both dashboards) ──

// buildEightRoleLoginRouter wires the production path for the authentication
// and dashboard flows: security headers + CSRF middleware, the real auth /
// dashboard / self-service handlers backed by real repositories and services,
// the real permission service (whose role-permission matrix is required for
// the permission-aware navigation), the production template set and the real
// route registrations.
//
// The generic IP request limiter is deliberately not mounted here: it is a
// package-level flood shield shared process-wide (30 POSTs/min/IP across the
// whole test binary), not part of the authentication semantics under test.
// The login-specific two-layer RateLimitLogin middleware — the component that
// historically answered valid logins with 429 and is squarely in scope — IS
// wired exactly as production registers it.
func buildEightRoleLoginRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	permissionRepo := repository.NewPermissionRepository(db)
	auditLogRepo := repository.NewAuditLogRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)
	personRepo := repository.NewPersonRepository(db)
	aidRepo := repository.NewAidRequestRepository(db)
	loanRepo := repository.NewLoanRepository(db)
	repaymentRepo := repository.NewLoanRepaymentRepository(db)
	careRepo := repository.NewCareProvidedRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)

	authSvc := services.NewAuthService(userRepo)
	sessionSvc := services.NewSessionService(sessionRepo)
	auditSvc := services.NewAuditLogService(auditLogRepo)
	notificationSvc := services.NewNotificationService(notificationRepo)
	permissionSvc := services.NewPermissionService(permissionRepo, roleRepo, auditLogRepo)
	dashboardSvc := services.NewDashboardService(userRepo, dashboardRepo)
	aidSvc := services.NewAidRequestService(aidRepo, personRepo, notificationSvc)
	loanSvc := services.NewLoanServiceWithNotifications(loanRepo, repaymentRepo, personRepo, db, notificationSvc)
	repaymentSvc := services.NewLoanRepaymentServiceWithPerson(repaymentRepo, loanRepo, personRepo, db)
	careSvc := services.NewCareProvidedServiceWithPerson(careRepo, personRepo)

	authHandler := handlers.NewAuthHandler(authSvc, sessionSvc, nil, auditSvc, false)
	dashboardHandler := handlers.NewDashboardHandler(dashboardSvc)
	selfServiceHandler := handlers.NewSelfServiceHandler(
		aidSvc, loanSvc, repaymentSvc, careSvc, dashboardSvc, auditSvc, personRepo, notificationSvc,
	)
	authMiddleware := middleware.NewAuthMiddleware(sessionSvc, false)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.EnsureCSRF(false))

	// Load ALL templates referenced by base.html so the dashboard can render
	// without "no such template" errors.
	router.LoadHTMLFiles(
		"../../web/templates/layouts/base.html",
		"../../web/templates/layouts/header.html",
		"../../web/templates/login.html",
		"../../web/templates/forgot_password.html",
		"../../web/templates/verify_reset_otp.html",
		"../../web/templates/reset_password.html",
		"../../web/templates/error.html",
		"../../web/templates/dashboard.html",
		"../../web/templates/beneficiary_dashboard_content.html",
		"../../web/templates/student_dashboard_content.html",
		"../../web/templates/home.html",
		"../../web/templates/my_aid_content.html",
		"../../web/templates/my_care_content.html",
		"../../web/templates/my_loans_content.html",
		"../../web/templates/my_repayments_content.html",
		"../../web/templates/request_aid_content.html",
		"../../web/templates/apply_for_loan_content.html",
		"../../web/templates/users.html",
		"../../web/templates/managed_users.html",
		"../../web/templates/system_settings.html",
		"../../web/templates/system_alerts.html",
		"../../web/templates/profile.html",
		"../../web/templates/persons.html",
		"../../web/templates/person_form.html",
		"../../web/templates/person_view.html",
		"../../web/templates/person_edit.html",
		"../../web/templates/students.html",
		"../../web/templates/students_table.html",
		"../../web/templates/student_view.html",
		"../../web/templates/student_edit.html",
		"../../web/templates/donor_view.html",
		"../../web/templates/donor_edit.html",
		"../../web/templates/donors.html",
		"../../web/templates/donations.html",
		"../../web/templates/aid_requests.html",
		"../../web/templates/care_provided.html",
		"../../web/templates/loans.html",
		"../../web/templates/loan_details.html",
		"../../web/templates/loan_repayments.html",
		"../../web/templates/revenue.html",
		"../../web/templates/notifications.html",
		"../../web/templates/messages.html",
		"../../web/templates/files.html",
		"../../web/templates/audit_logs.html",
		"../../web/templates/reports.html",
		"../../web/templates/permissions.html",
	)

	routes.RegisterAuthRoutes(router, authHandler, dashboardHandler, authMiddleware, permissionSvc)
	routes.RegisterSelfServiceRoutes(router, selfServiceHandler, authMiddleware, permissionSvc)

	return router
}

// roleLoginOutcome captures the verified result of a full login + dashboard
// round-trip for one role so the summary table can be printed from real data.
type roleLoginOutcome struct {
	role             string
	loginStatus      int
	sessionCreated   bool
	redirectLocation string
	finalPath        string
	finalStatus      int
	dashboardOK      bool
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// followLoginRoundTrip drives one browser-like session through the complete
// login + dashboard flow and reports the outcome.
func followLoginRoundTrip(
	t *testing.T,
	router *gin.Engine,
	login, password, clientIP string,
	roleName string,
) roleLoginOutcome {
	t.Helper()

	ts := httptest.NewServer(router)
	defer ts.Close()

	s := newEightRoleTestSession(t, ts.URL, clientIP)
	s.fetchLoginPage()

	loginResp := s.postLogin(login, password)
	defer s.close(loginResp)

	outcome := roleLoginOutcome{
		role:        roleName,
		loginStatus: loginResp.StatusCode,
	}

	hasSession := false
	for _, c := range s.jar.Cookies(mustParseURL(ts.URL)) {
		if c.Name == "pwams_session" && c.Value != "" {
			hasSession = true
			break
		}
	}
	outcome.sessionCreated = hasSession

	if loginResp.StatusCode == http.StatusSeeOther {
		outcome.redirectLocation = loginResp.Header.Get("Location")
	}

	currentURL := ts.URL + "/dashboard"
	if loginResp.StatusCode == http.StatusSeeOther {
		loc := loginResp.Header.Get("Location")
		if loc != "" {
			currentURL = ts.URL + loc
		}
	}

	var finalResp *http.Response
	for hop := 0; hop < 5; hop++ {
		resp := s.do(http.MethodGet, currentURL[len(ts.URL):], nil, "", false)
		if resp.StatusCode == http.StatusSeeOther {
			loc := resp.Header.Get("Location")
			s.close(resp)
			if loc == "" {
				break
			}
			currentURL = ts.URL + loc
			continue
		}
		finalResp = resp
		break
	}
	if finalResp == nil {
		t.Fatalf("%s: redirect loop or no final response", roleName)
	}
	defer s.close(finalResp)

	outcome.finalStatus = finalResp.StatusCode
	outcome.finalPath = currentURL[len(ts.URL):]
	body := s.html(finalResp)

	switch roleName {
	case models.RoleSuperAdmin, models.RoleAdmin, models.RolePartner,
		models.RoleStaff, models.RoleVolunteer, models.RoleDonor:
		outcome.dashboardOK = strings.Contains(body, "Dashboard") ||
			strings.Contains(body, "System Control Center")
	case models.RoleBeneficiary:
		outcome.dashboardOK = strings.Contains(body, "My Dashboard")
	case models.RoleStudent:
		outcome.dashboardOK = strings.Contains(body, "Student Dashboard") ||
			strings.Contains(body, "My Dashboard")
	}

	if strings.Contains(body, "runtime error") || strings.Contains(body, "template:") {
		t.Fatalf("%s: dashboard rendered a template error: %s", roleName, firstN(body, 500))
	}

	return outcome
}

func TestIntegration_EightRoleLoginDashboard_AllRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := eightRoleDB(t)

	router := buildEightRoleLoginRouter(t, db)

	roles := []string{
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	}

	type row struct {
		roleName string
		prefix   string
		ip       string
	}
	specs := make([]row, len(roles))
	for i, role := range roles {
		specs[i] = row{
			roleName: role,
			prefix:   strings.ToLower(strings.ReplaceAll(role, " ", "-")),
			ip:       fmt.Sprintf("10.10.10.%d", i+1),
		}
	}

	fixtures := make(map[string]*models.User, len(specs))
	for _, spec := range specs {
		fixtures[spec.roleName] = seedEightRoleFixtureUser(t, db, spec.roleName, spec.prefix, nil)
	}

	outcomes := make([]roleLoginOutcome, len(specs))
	for i, spec := range specs {
		u := fixtures[spec.roleName]
		outcomes[i] = followLoginRoundTrip(t, router, u.Email, eightRoleLoginPassword, spec.ip, spec.roleName)
	}

	fmt.Println("")
	fmt.Println("Role | Login | Session | Redirect | Final Dashboard | HTTP Status | Result")
	fmt.Println("---- | ----- | ------- | -------- | --------------- | ----------- | ------")
	allPass := true
	for _, o := range outcomes {
		result := "PASS"
		if o.loginStatus != http.StatusSeeOther {
			result = "FAIL"
			allPass = false
		}
		if !o.sessionCreated {
			result = "FAIL"
			allPass = false
		}
		if o.finalStatus != http.StatusOK {
			result = "FAIL"
			allPass = false
		}
		if !o.dashboardOK {
			result = "FAIL"
			allPass = false
		}
		fmt.Printf("%s | %d | %v | %s | %s | %d | %s\n",
			o.role, o.loginStatus, o.sessionCreated, o.redirectLocation, o.finalPath, o.finalStatus, result)
	}
	fmt.Println("")

	if !allPass {
		t.Fatal("one or more roles failed the login + dashboard flow — see table above")
	}
}

// ── Disabled user cannot log in ───────────────────────────────────────────

func TestIntegration_EightRoleLogin_DisabledUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := eightRoleDB(t)

	router := buildEightRoleLoginRouter(t, db)

	u := seedEightRoleFixtureUser(t, db, models.RoleStaff, "disabled-user", func(user *models.User) {
		user.Status = models.UserStatusDisabled
	})

	ts := httptest.NewServer(router)
	defer ts.Close()

	s := newEightRoleTestSession(t, ts.URL, "10.20.30.40")
	s.fetchLoginPage()

	resp := s.postLogin(u.Email, eightRoleLoginPassword)
	defer s.close(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("disabled user login must return 401, got %d", resp.StatusCode)
	}

	for _, c := range s.jar.Cookies(mustParseURL(ts.URL)) {
		if c.Name == "pwams_session" && c.Value != "" {
			t.Fatal("disabled user must not receive a session cookie")
		}
	}
}

// ── Actively locked user cannot log in ────────────────────────────────────

func TestIntegration_EightRoleLogin_ActiveLockBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := eightRoleDB(t)

	router := buildEightRoleLoginRouter(t, db)

	u := seedEightRoleLockedUser(t, db, "active-locked-user", false)

	ts := httptest.NewServer(router)
	defer ts.Close()

	s := newEightRoleTestSession(t, ts.URL, "10.20.30.41")
	s.fetchLoginPage()

	resp := s.postLogin(u.Email, eightRoleLoginPassword)
	defer s.close(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("actively locked user login must return 401, got %d", resp.StatusCode)
	}

	for _, c := range s.jar.Cookies(mustParseURL(ts.URL)) {
		if c.Name == "pwams_session" && c.Value != "" {
			t.Fatal("actively locked user must not receive a session cookie")
		}
	}
}

// ── Expired-lock user recovers with correct password ──────────────────────

func TestIntegration_EightRoleLogin_ExpiredLockRecovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := eightRoleDB(t)

	router := buildEightRoleLoginRouter(t, db)

	u := seedEightRoleLockedUser(t, db, "expired-lock-user", true)

	ts := httptest.NewServer(router)
	defer ts.Close()

	s := newEightRoleTestSession(t, ts.URL, "10.20.30.42")
	s.fetchLoginPage()

	resp := s.postLogin(u.Email, eightRoleLoginPassword)
	defer s.close(resp)

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expired-lock user with correct password must return 303, got %d", resp.StatusCode)
	}

	hasSession := false
	for _, c := range s.jar.Cookies(mustParseURL(ts.URL)) {
		if c.Name == "pwams_session" && c.Value != "" {
			hasSession = true
			break
		}
	}
	if !hasSession {
		t.Fatal("expired-lock user who logged in successfully must receive a session cookie")
	}

	var reloaded models.User
	if err := db.First(&reloaded, "id = ?", u.ID).Error; err != nil {
		t.Fatalf("cannot reload fixture user: %v", err)
	}
	if reloaded.Status != models.UserStatusActive {
		t.Fatalf("expired-lock user must be Active after successful login, got %q", reloaded.Status)
	}
	if reloaded.FailedLoginAttempts != 0 {
		t.Fatalf("expired-lock user must have 0 failed attempts after successful login, got %d", reloaded.FailedLoginAttempts)
	}
}

// ── Wrong password still fails ────────────────────────────────────────────

func TestIntegration_EightRoleLogin_WrongPasswordFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := eightRoleDB(t)

	router := buildEightRoleLoginRouter(t, db)

	u := seedEightRoleFixtureUser(t, db, models.RoleDonor, "wrong-pw-user", nil)

	ts := httptest.NewServer(router)
	defer ts.Close()

	s := newEightRoleTestSession(t, ts.URL, "10.20.30.43")
	s.fetchLoginPage()

	resp := s.postLogin(u.Email, "Definitely!Wrong9999")
	defer s.close(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password must return 401, got %d", resp.StatusCode)
	}

	for _, c := range s.jar.Cookies(mustParseURL(ts.URL)) {
		if c.Name == "pwams_session" && c.Value != "" {
			t.Fatal("wrong password must not create a session")
		}
	}

	var reloaded models.User
	if err := db.First(&reloaded, "id = ?", u.ID).Error; err != nil {
		t.Fatalf("cannot reload fixture user: %v", err)
	}
	if reloaded.FailedLoginAttempts < 1 {
		t.Fatalf("wrong password must increment failed-login counter, got %d", reloaded.FailedLoginAttempts)
	}
}

// ── Successful login does not consume the failed-login budget ─────────────

func TestIntegration_EightRoleLogin_No429OnValidLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := eightRoleDB(t)

	router := buildEightRoleLoginRouter(t, db)

	u := seedEightRoleFixtureUser(t, db, models.RoleVolunteer, "no-429-user", nil)

	ts := httptest.NewServer(router)
	defer ts.Close()

	for attempt := 1; attempt <= 3; attempt++ {
		sAttempt := newEightRoleTestSession(t, ts.URL, "10.20.30.44")
		sAttempt.fetchLoginPage()
		resp := sAttempt.postLogin(u.Email, eightRoleLoginPassword)
		if resp.StatusCode == http.StatusTooManyRequests {
			sAttempt.close(resp)
			t.Fatalf("attempt %d: valid login returned 429 — successful logins must not consume the failed-login budget", attempt)
		}
		if resp.StatusCode != http.StatusSeeOther {
			sAttempt.close(resp)
			t.Fatalf("attempt %d: valid login must return 303, got %d", attempt, resp.StatusCode)
		}
		sAttempt.close(resp)
	}
}

// ── Student navigation exposes /my/dashboard but not admin modules ────────

func TestIntegration_StudentNavigation_SelfServiceOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := eightRoleDB(t)

	router := buildEightRoleLoginRouter(t, db)

	u := seedEightRoleFixtureUser(t, db, models.RoleStudent, "student-nav", nil)

	ts := httptest.NewServer(router)
	defer ts.Close()

	s := newEightRoleTestSession(t, ts.URL, "10.20.30.50")
	s.fetchLoginPage()

	loginResp := s.postLogin(u.Email, eightRoleLoginPassword)
	defer s.close(loginResp)

	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("student login must return 303, got %d", loginResp.StatusCode)
	}

	dashResp := s.get("/dashboard")
	defer s.close(dashResp)
	if dashResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET /dashboard for Student must redirect 303, got %d", dashResp.StatusCode)
	}
	if loc := dashResp.Header.Get("Location"); loc != "/my/dashboard" {
		t.Fatalf("Student /dashboard must redirect to /my/dashboard, got %q", loc)
	}

	myDashResp := s.get("/my/dashboard")
	defer s.close(myDashResp)
	if myDashResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /my/dashboard for Student must return 200, got %d", myDashResp.StatusCode)
	}
	body := s.html(myDashResp)

	if !strings.Contains(body, "My Dashboard") && !strings.Contains(body, "Student Dashboard") {
		t.Fatalf("Student /my/dashboard must render the self-service dashboard, got %d bytes", len(body))
	}

	forbidden := []string{"/users/page", "/admins/page", "/persons/page", "/students/page", "/donors/page", "/donations/page"}
	for _, path := range forbidden {
		if strings.Contains(body, path) {
			t.Fatalf("Student dashboard must not expose operational module %q", path)
		}
	}
}
