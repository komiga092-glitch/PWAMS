package routes_test

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/routes"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// ─────────────────────────────────────────────────────────────────────────────
// Live integration test database (shared, initialised once per binary).
// The same .env-driven database the existing services integration tests use.
// No destructive SQL: tests only create their own probe users/sessions and
// delete exactly those records afterwards.
// ─────────────────────────────────────────────────────────────────────────────

var (
	reportTestDB     *gorm.DB
	reportTestDBOnce sync.Once
	reportTestDBErr  error
)

func acquireReportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	reportTestDBOnce.Do(func() {
		// config.Load() reads godotenv from the CWD; test binaries run from
		// their package directory, so the repo-root .env is loaded here as a
		// fallback. Existing variables are never overridden.
		_ = godotenv.Load("../../.env")

		cfg, err := config.Load()
		if err != nil {
			reportTestDBErr = err
			return
		}

		reportTestDB, reportTestDBErr = database.Connect(cfg)
		if reportTestDBErr != nil {
			return
		}

		if err := database.Migrate(reportTestDB); err != nil {
			reportTestDBErr = err
			return
		}
		if err := database.SeedDefaultRoles(reportTestDB); err != nil {
			reportTestDBErr = err
		}
	})

	if reportTestDBErr != nil {
		t.Skipf("integration test DB unavailable: %v", reportTestDBErr)
	}
	if reportTestDB == nil {
		t.Skip("integration test DB unavailable: connection is nil")
	}

	return reportTestDB
}

func newReportTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)

	sessionService := services.NewSessionService(repository.NewSessionRepository(db))
	authMiddleware := middleware.NewAuthMiddleware(sessionService)

	reportService := services.NewReportService(repository.NewReportRepository(db))

	router := gin.New()
	// The authoritative template set, the same one cmd/server loads. Loading
	// anything less makes html/template fail to resolve base.html's dispatch
	// chain (`home_content` etc.) and every page silently renders a zero-byte
	// body with HTTP 200.
	router.SetFuncMap(template.FuncMap(routes.TemplateFuncMap()))
	router.LoadHTMLFiles(routes.TemplateFilesUnder("../../")...)
	routes.RegisterReportRoutes(
		router,
		handlers.NewReportHandler(reportService, services.NewReportPDFService()),
		authMiddleware,
	)
	return router
}

// createReportProbeUser creates a throwaway user for one role. Cleanup
// deletes ONLY the records this test created (its sessions + its user row).
func createReportProbeUser(t *testing.T, db *gorm.DB, roleName string) *models.User {
	t.Helper()

	var role models.Role
	if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
		t.Fatalf("role %q not found: %v", roleName, err)
	}

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	user := &models.User{
		Username:     "rpt_probe_" + stamp,
		Email:        "rpt_probe_" + stamp + "@example.test",
		PasswordHash: "probe-not-a-real-credential",
		RoleID:       role.ID,
		Status:       models.UserStatusActive,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create probe user: %v", err)
	}

	t.Cleanup(func() {
		db.Where("user_id = ?", user.ID).Delete(&models.Session{})
		db.Unscoped().Delete(&models.User{}, "id = ?", user.ID)
	})

	return user
}

func reportSessionToken(t *testing.T, db *gorm.DB, user *models.User) string {
	t.Helper()

	sessionService := services.NewSessionService(repository.NewSessionRepository(db))
	token, _, err := sessionService.CreateSession(user.ID)
	if err != nil {
		t.Fatalf("failed to create probe session: %v", err)
	}

	t.Cleanup(func() {
		_ = sessionService.RevokeSession(token)
	})

	return token
}

// reportDo performs one authenticated GET. Browser mode sets the HTML Accept
// header (as a real page navigation would); API mode asks for JSON.
func reportDo(router *gin.Engine, target, token string, browser bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "pwams_session", Value: token})
	}
	if browser {
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
	} else {
		req.Header.Set("Accept", "application/json")
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// roleAllowedForReport mirrors the backend authorization implemented in
// internal/routes/report_routes.go so the matrix is explicit per role.
func roleAllowedForReport(def handlers.ReportDef, role string) bool {
	switch role {
	case models.RoleSuperAdmin, models.RoleAdmin:
		return true
	case models.RoleManager, models.RoleStaff:
		return !def.PlatformOnly
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// §16/§22: role × surface authorization matrix (backend, not UI hiding)
// ─────────────────────────────────────────────────────────────────────────────

func TestReportRoutesRoleMatrix(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	roles := []string{
		models.RoleSuperAdmin, models.RoleAdmin, models.RoleManager, models.RoleStaff,
		models.RoleVolunteer, models.RoleDonor, models.RoleBeneficiary, models.RoleStudent,
	}

	for _, roleName := range roles {
		t.Run(roleName, func(t *testing.T) {
			user := createReportProbeUser(t, db, roleName)
			token := reportSessionToken(t, db, user)

			for _, def := range handlers.ReportRegistry() {
				allowed := roleAllowedForReport(def, roleName)

				t.Run(def.Slug+"/page", func(t *testing.T) {
					rec := reportDo(router, "/reports/"+def.Slug+"/page", token, true)
					if allowed {
						if rec.Code != http.StatusOK {
							t.Errorf("expected 200 HTML, got %d", rec.Code)
						}
						if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
							t.Errorf("HTML route returned Content-Type %q", ct)
						}
					} else if rec.Code != http.StatusForbidden {
						t.Errorf("expected 403 for %s on %s page, got %d", roleName, def.Slug, rec.Code)
					}
				})

				t.Run(def.Slug+"/api", func(t *testing.T) {
					rec := reportDo(router, "/api/reports/"+def.Slug, token, false)
					if allowed {
						if rec.Code != http.StatusOK {
							t.Errorf("expected 200 JSON, got %d (body: %.200s)", rec.Code, rec.Body.String())
						}
						if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
							t.Errorf("JSON route returned Content-Type %q", ct)
						}
					} else if rec.Code != http.StatusForbidden {
						t.Errorf("expected 403 for %s on %s API, got %d", roleName, def.Slug, rec.Code)
					}
				})

				t.Run(def.Slug+"/pdf", func(t *testing.T) {
					rec := reportDo(router, "/reports/"+def.Slug+"/pdf", token, false)
					if allowed {
						if rec.Code != http.StatusOK {
							t.Errorf("expected 200 PDF, got %d", rec.Code)
						}
						if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/pdf") {
							t.Errorf("PDF route returned Content-Type %q", ct)
						}
						if !strings.HasPrefix(rec.Body.String(), "%PDF-") {
							t.Error("PDF route body does not start with %PDF-")
						}
						if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment; filename=") {
							t.Errorf("PDF route Content-Disposition=%q", cd)
						}
					} else if rec.Code != http.StatusForbidden {
						t.Errorf("expected 403 for %s on %s PDF, got %d", roleName, def.Slug, rec.Code)
					}
				})
			}

			// Landing page: operational group, same rule as operational reports.
			landingAllowed := roleAllowedForReport(handlers.ReportDef{}, roleName)
			landing := reportDo(router, "/reports/page", token, true)
			if landingAllowed {
				if landing.Code != http.StatusOK {
					t.Errorf("landing page: expected 200, got %d", landing.Code)
				}
			} else if landing.Code != http.StatusForbidden {
				t.Errorf("landing page: expected 403 for %s, got %d", roleName, landing.Code)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §5/§6/§7: landing cards, detail page content, filters, empty state
// ─────────────────────────────────────────────────────────────────────────────

func TestReportsLandingPageRendersCardsWithLiveLinks(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	rec := reportDo(router, "/reports/page", token, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("landing page: status=%d", rec.Code)
	}
	body := rec.Body.String()

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("landing Content-Type=%q", ct)
	}

	for _, needle := range []string{
		"/reports/dashboard/page",
		"/reports/persons/page",
		"/reports/students/page",
		"/reports/donors/page",
		"/reports/users/page",
		"/reports/aid-requests/page",
		"/reports/care-provided/page",
		"/reports/donations/page",
		"/reports/loans/page",
		"/reports/loan-repayments/page",
		"/reports/revenue/page",
		"/reports/account-status/page",
		"/reports/audit-log/page",
		"/reports/system-alerts/page",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("landing page is missing card link %s", needle)
		}
	}

	// Platform-only cards must be UI-hidden for non-admin roles.
	if !strings.Contains(body, `data-roles="Super Admin,Admin"`) {
		t.Error("landing page platform cards lack data-roles hiding")
	}

	// No raw JSON endpoint may appear as a navigation target.
	if strings.Contains(body, `href="/reports/dashboard"`) {
		t.Error("landing page links to the raw JSON endpoint /reports/dashboard")
	}
}

func TestReportsDonationDetailPageContent(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	rec := reportDo(router, "/reports/donations/page", token, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("donations page: status=%d", rec.Code)
	}
	body := rec.Body.String()

	for _, needle := range []string{
		"Donations Report",
		"Total Donations",
		"Total Amount",
		"Detailed Records",
		`href="/reports/page"`,
		`href="/reports/donations/pdf"`,
		`name="from"`,
		`name="to"`,
		`name="status"`,
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("donations page missing %q", needle)
		}
	}
}

func TestReportsDonationDetailPageFiltersAndEmptyState(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	// A filter that can never match → empty state, still HTTP 200 HTML.
	rec := reportDo(
		router,
		"/reports/donations/page?from=2001-01-01&to=2001-01-02&q=zzzz-no-such-record",
		token,
		true,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("filtered donations page: status=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("filtered donations page Content-Type=%q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "No records found.") {
		t.Error("filtered donations page does not show the empty state")
	}

	// Date filter that matches everything must still render 200.
	rec2 := reportDo(router, "/reports/donations/page?from=2000-01-01&to=2100-01-01", token, true)
	if rec2.Code != http.StatusOK {
		t.Errorf("wide date filter: status=%d", rec2.Code)
	}

	// Malformed date must not 500 (validated + parameterised).
	rec3 := reportDo(router, "/reports/donations/page?from=not-a-date", token, true)
	if rec3.Code >= http.StatusInternalServerError {
		t.Errorf("malformed date: status=%d", rec3.Code)
	}
}

// RPT-003: a report reflects only the data inside the specified range/filter, so
// the summary cards must be narrowed by exactly the same filter as the detail
// table. Cards that ignore the filter are a defect even when the rows are
// correct — the two surfaces would disagree about the same report.
func TestReportsSummaryHonoursDetailFilter(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	// A probe donation whose item name is unique, so a text filter matches
	// exactly one row and the expected figures are unambiguous.
	stamp := fmt.Sprintf("rpt_summary_%d", time.Now().UnixNano())

	donor := &models.Donor{
		Name:        "Probe Donor " + stamp,
		DonorType:   models.DonorTypeIndividual,
		Status:      models.DonorStatusActive,
		CreatedByID: user.ID,
	}
	if err := db.Create(donor).Error; err != nil {
		t.Fatalf("failed to create probe donor: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&models.Donor{}, "id = ?", donor.ID) })

	donation := &models.Donation{
		DonorID:      donor.ID,
		DonationType: models.DonationTypeCash,
		Amount:       decimal.NewFromInt(1234),
		Currency:     "LKR",
		ItemName:     stamp,
		DonationDate: time.Now().UTC(),
		Status:       models.DonationStatusConfirmed,
		CreatedByID:  user.ID,
	}
	if err := db.Create(donation).Error; err != nil {
		t.Fatalf("failed to create probe donation: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&models.Donation{}, "id = ?", donation.ID) })

	type reportPayload struct {
		Success    bool           `json:"success"`
		Summary    map[string]any `json:"summary"`
		Pagination struct {
			TotalItems int64 `json:"total_items"`
		} `json:"pagination"`
	}

	load := func(target string) reportPayload {
		rec := reportDo(router, target, token, false)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%.200s", target, rec.Code, rec.Body.String())
		}

		var payload reportPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("GET %s: invalid JSON: %v", target, err)
		}

		return payload
	}

	filtered := load("/api/reports/donations?q=" + stamp)
	if filtered.Pagination.TotalItems != 1 {
		t.Fatalf("filtered detail rows=%d, want 1", filtered.Pagination.TotalItems)
	}
	if got := filtered.Summary["total_donations"]; got != float64(1) {
		t.Errorf("summary.total_donations=%v with the filter applied, want 1 — the "+
			"summary must count the same rows the detail table shows", got)
	}
	if got := filtered.Summary["total_amount"]; got != float64(1234) {
		t.Errorf("summary.total_amount=%v with the filter applied, want 1234", got)
	}

	// A filter that matches nothing must zero the cards too.
	empty := load("/api/reports/donations?q=" + stamp + "-no-such-record")
	if empty.Pagination.TotalItems != 0 {
		t.Fatalf("unmatched filter detail rows=%d, want 0", empty.Pagination.TotalItems)
	}
	if got := empty.Summary["total_donations"]; got != float64(0) {
		t.Errorf("summary.total_donations=%v for a filter that matches nothing, want 0", got)
	}
	if got := empty.Summary["total_amount"]; got != float64(0) {
		t.Errorf("summary.total_amount=%v for a filter that matches nothing, want 0", got)
	}

	// The detail page renders the same scoped figures as the JSON API.
	pageRec := reportDo(router, "/reports/donations/page?q="+stamp, token, true)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("filtered donations page: status=%d", pageRec.Code)
	}
	if body := pageRec.Body.String(); !strings.Contains(body, "1234.00") {
		t.Error("filtered donations page does not render the filtered summary total 1234.00")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §12/§13: JSON structure + pagination contract
// ─────────────────────────────────────────────────────────────────────────────

func TestReportsJSONResponseStructure(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	rec := reportDo(router, "/api/reports/donations?page=1&page_size=2", token, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("donations API: status=%d body=%.200s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("donations API Content-Type=%q", ct)
	}

	var payload struct {
		Success    bool `json:"success"`
		Summary    map[string]any
		Data       []map[string]any
		Pagination *struct {
			Page       int    `json:"page"`
			PageSize   int    `json:"page_size"`
			TotalItems int64  `json:"total_items"`
			TotalPages int    `json:"total_pages"`
			HasPrev    bool   `json:"has_prev"`
			HasNext    bool   `json:"has_next"`
			PrevURL    string `json:"prev_url"`
			NextURL    string `json:"next_url"`
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("donations API payload is not valid JSON: %v", err)
	}

	if !payload.Success {
		t.Error("donations API success=false")
	}
	if len(payload.Summary) == 0 {
		t.Error("donations API missing summary object")
	}
	if payload.Data == nil {
		t.Error("donations API missing data array")
	}
	if payload.Pagination == nil {
		t.Fatal("donations API missing pagination object")
	}
	if payload.Pagination.Page != 1 || payload.Pagination.PageSize != 2 {
		t.Errorf("pagination echo wrong: %+v", payload.Pagination)
	}
	if payload.Pagination.TotalItems > 2 && payload.Pagination.TotalPages < 2 {
		t.Errorf("total_pages inconsistent with total_items: %+v", payload.Pagination)
	}

	// Aggregate-only report: summary only, no data/pagination keys.
	rec2 := reportDo(router, "/api/reports/system-alerts", token, false)
	if rec2.Code != http.StatusOK {
		t.Fatalf("system-alerts API: status=%d", rec2.Code)
	}
	var alerts map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &alerts); err != nil {
		t.Fatalf("system-alerts API payload invalid: %v", err)
	}
	if alerts["success"] != true {
		t.Error("system-alerts API success=false")
	}
	if _, ok := alerts["summary"]; !ok {
		t.Error("system-alerts API missing summary")
	}
}

func TestReportsPaginationRendersAndPreservesFilters(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	rec := reportDo(
		router,
		"/api/reports/donations?page=1&page_size=2&status=Confirmed",
		token,
		false,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("donations API (filtered): status=%d", rec.Code)
	}

	var payload struct {
		Pagination *struct {
			TotalItems int64  `json:"total_items"`
			HasNext    bool   `json:"has_next"`
			NextURL    string `json:"next_url"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("filtered API payload invalid: %v", err)
	}
	if payload.Pagination == nil {
		t.Fatal("pagination missing")
	}

	// If more pages exist, the next URL must preserve the active filter.
	if payload.Pagination.HasNext && !strings.Contains(payload.Pagination.NextURL, "status=Confirmed") {
		t.Errorf("next_url loses the status filter: %q", payload.Pagination.NextURL)
	}

	// HTML pagination controls render when rows exist.
	htmlRec := reportDo(router, "/reports/donations/page?page=1&page_size=2", token, true)
	if htmlRec.Code != http.StatusOK {
		t.Fatalf("donations page (paged): status=%d", htmlRec.Code)
	}
	if payload.Pagination.TotalItems > 2 {
		body := htmlRec.Body.String()
		if !strings.Contains(body, "Page 1") || !strings.Contains(body, "of") {
			t.Error("paged donations page missing the pagination controls")
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §17: audit report must never expose IP addresses
// ─────────────────────────────────────────────────────────────────────────────

var reportIPv4Pattern = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)

func TestReportsAuditLogContainsNoIPAddress(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	for _, target := range []string{"/api/reports/audit-log", "/reports/audit-log/page"} {
		browser := strings.Contains(target, "/page")
		rec := reportDo(router, target, token, browser)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d", target, rec.Code)
		}

		body := rec.Body.String()
		if strings.Contains(strings.ToLower(body), "ip_address") {
			t.Errorf("%s contains ip_address", target)
		}
		if match := reportIPv4Pattern.FindString(body); match != "" {
			t.Errorf("%s appears to contain an IPv4 address: %q", target, match)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §15: PDF contract details (filename + correct content type)
// ─────────────────────────────────────────────────────────────────────────────

func TestReportsPDFContract(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	rec := reportDo(router, "/reports/donations/pdf", token, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("donations PDF: status=%d", rec.Code)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment; filename=") {
		t.Errorf("donations PDF Content-Disposition=%q", cd)
	}
	if !regexp.MustCompile(`filename="donations-report-\d{4}-\d{2}-\d{2}\.pdf"`).MatchString(cd) {
		t.Errorf("donations PDF filename wrong: %q", cd)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/pdf") {
		t.Errorf("donations PDF Content-Type=%q", ct)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §18: language switching on report pages (no raw keys, no unrendered actions)
// ─────────────────────────────────────────────────────────────────────────────

func TestReportsRenderInAllLanguages(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	for _, lang := range []string{"en", "ta", "si"} {
		rec := reportDo(router, "/reports/donations/page?lang="+lang, token, true)
		if rec.Code != http.StatusOK {
			t.Errorf("lang=%s: status=%d", lang, rec.Code)
			continue
		}
		body := rec.Body.String()

		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html; charset=utf-8") {
			t.Errorf("lang=%s: Content-Type=%q", lang, ct)
		}
		// A missing translation falls back to English, never to the raw key.
		if strings.Contains(body, "reports.donations_report") {
			t.Errorf("lang=%s: raw translation key leaked into the page", lang)
		}
		if strings.Contains(body, "{{t") {
			t.Errorf("lang=%s: unprocessed template action in output", lang)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Sessions for non-active accounts must never reach report data
// ─────────────────────────────────────────────────────────────────────────────

func TestReportRoutesRejectInactiveUserSession(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newReportTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	// Deactivate AFTER the session was minted; ValidateSession must refuse it.
	if err := db.Model(&models.User{}).Where("id = ?", user.ID).
		Update("status", models.UserStatusDisabled).Error; err != nil {
		t.Fatalf("failed to disable probe user: %v", err)
	}

	rec := reportDo(router, "/api/reports/donations", token, false)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("disabled user session: status=%d, want 401", rec.Code)
	}
}
