package routes_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	texttemplate "text/template"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/routes"
)

// buildReportTestRouter registers ONLY the report routes against an engine
// whose auth middleware has no backing session store, so every request is
// rejected (401/redirect) before reaching a handler. This proves the
// registration shape without needing a database.
func buildReportTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	routes.RegisterReportRoutes(
		router,
		handlers.NewReportHandler(nil, nil),
		middleware.NewAuthMiddleware(nil),
	)
	return router
}

func registeredReportPaths(router *gin.Engine) map[string]bool {
	paths := map[string]bool{}
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet {
			paths[route.Path] = true
		}
	}
	return paths
}

// ─────────────────────────────────────────────────────────────────────────────
// §21 route registry: every report area has page + JSON + PDF routes
// ─────────────────────────────────────────────────────────────────────────────

func TestReportRegistryRegistersAllSurfaces(t *testing.T) {
	router := buildReportTestRouter()
	paths := registeredReportPaths(router)

	if !paths["/reports/page"] {
		t.Error("landing route /reports/page is not registered")
	}

	seenSlugs := map[string]bool{}

	for _, def := range handlers.ReportRegistry() {
		if seenSlugs[def.Slug] {
			t.Errorf("duplicate report slug in registry: %s", def.Slug)
		}
		seenSlugs[def.Slug] = true

		for _, path := range []string{
			"/reports/" + def.Slug + "/page",
			"/api/reports/" + def.Slug,
			"/reports/" + def.Slug + "/pdf",
		} {
			if !paths[path] {
				t.Errorf("report %q: route %s is not registered", def.Slug, path)
			}
		}
	}

	// All 14 required report areas must be present.
	for _, slug := range []string{
		"dashboard", "users", "persons", "students", "donors", "donations",
		"aid-requests", "care-provided", "loans", "loan-repayments",
		"revenue", "account-status", "audit-log", "system-alerts",
	} {
		if !seenSlugs[slug] {
			t.Errorf("required report area missing from registry: %s", slug)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §21: every registered report page renders an existing template block
// ─────────────────────────────────────────────────────────────────────────────

func loadAppTemplates(t *testing.T) *texttemplate.Template {
	t.Helper()

	files := routes.TemplateFilesUnder("../../")

	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("template missing: %s (%v)", f, err)
		}
	}

	funcs := texttemplate.FuncMap(routes.TemplateFuncMap())

	tmpl, err := texttemplate.New("app").Funcs(funcs).ParseFiles(files...)
	if err != nil {
		t.Fatalf("failed to parse app templates: %v", err)
	}
	return tmpl
}

func reportSupersetPageData() map[string]any {
	countFields := []string{
		"TotalUsers", "TotalPersons", "TotalStudents", "TotalDonors", "TotalDonations",
		"PendingDonations", "ConfirmedDonations", "CancelledDonations",
		"TotalAidRequests", "TotalCareProvided",
		"ActiveUsers", "DisabledUsers", "LockedUsers", "NewLast30Days",
		"ActivePersons", "InactivePersons", "PendingPersons",
		"ActiveStudents", "InactiveStudents", "PendingStudents", "RecentStudents",
		"ActiveDonors", "InactiveDonors",
		"TotalRequests", "PendingRequests", "UnderReviewCount",
		"ApprovedRequests", "RejectedRequests", "CompletedRequests",
		"TotalRecords", "CompletedRecords", "PendingRecords", "CancelledRecords",
		"ActiveLoans", "PendingLoans", "ApprovedLoans", "CompletedLoans",
		"TotalRepayments", "PaidRepayments", "PendingRepayments", "OverdueRepayments",
		"TotalEntries", "TotalAlerts", "FailedLogins", "LockedAccounts",
		"DeniedAccess", "RecentAlerts", "LockedNow", "NeverLoggedIn",
	}
	amountFields := []string{
		"TotalAmount", "TotalDonationAmount", "TotalDisbursed",
		"OutstandingAmount", "TotalPaidAmount", "TotalIncome", "TotalExpenses", "Net",
	}

	report := map[string]any{
		"ByStatus": map[string]int64{},
		"ByAction": map[string]int64{},
		"ByEntity": map[string]int64{},
	}
	for _, field := range countFields {
		report[field] = int64(1)
	}
	for _, field := range amountFields {
		report[field] = 1.5
	}

	return map[string]any{
		"Lang":             "en",
		"page_template":    "unused",
		"title":            "Test",
		"report_slug":      "donations",
		"report":           report,
		"filters":          models.ReportFilter{},
		"table_headers":    []string{"A", "B"},
		"table_rows":       [][]string{{"1", "2"}},
		"pagination":       &models.ReportPagination{Page: 1, TotalPages: 2, HasNext: true, NextURL: "?page=2"},
		"status_options":   []string{"Active"},
		"status_label_key": "common.status",
	}
}

func TestReportRegistryTemplatesExistAndExecute(t *testing.T) {
	tmpl := loadAppTemplates(t)

	for _, name := range []string{"base", "reports_content", "error.html"} {
		if tmpl.Lookup(name) == nil {
			t.Errorf("required template %q is not defined", name)
		}
	}

	pageData := reportSupersetPageData()

	for _, def := range handlers.ReportRegistry() {
		block := tmpl.Lookup(def.Template)
		if block == nil {
			t.Errorf("report %q: template block %q is not defined", def.Slug, def.Template)
			continue
		}

		// The page template name must follow the one convention.
		want := "report_" + strings.ReplaceAll(def.Slug, "-", "_") + "_content"
		if def.Template != want {
			t.Errorf("report %q: template %q does not follow the report_<slug>_content convention", def.Slug, def.Template)
		}

		clone, err := block.Clone()
		if err != nil {
			t.Fatalf("failed to clone %s: %v", def.Template, err)
		}
		if err := clone.Execute(io.Discard, pageData); err != nil {
			t.Errorf("report %q: template %q failed to execute: %v", def.Slug, def.Template, err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §20: every /reports… link in the UI points to a registered route
// ─────────────────────────────────────────────────────────────────────────────

func TestReportUILinksPointToRegisteredRoutes(t *testing.T) {
	router := buildReportTestRouter()
	paths := registeredReportPaths(router)

	allowed := map[string]bool{}
	for path := range paths {
		if strings.HasPrefix(path, "/reports") || strings.HasPrefix(path, "/api/reports") {
			allowed[path] = true
		}
	}
	allowed["/reports/page"] = true

	hrefPattern := regexp.MustCompile(`href="(/[^"#?]*)[^"]*"`)

	templates := []string{
		"../../web/templates/reports.html",
		"../../web/templates/report_detail.html",
		"../../web/templates/dashboard.html",
		"../../web/templates/layouts/header.html",
	}

	for _, name := range templates {
		raw, err := os.ReadFile(filepath.FromSlash(name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}

		for _, match := range hrefPattern.FindAllStringSubmatch(string(raw), -1) {
			href := match[1]
			if !strings.HasPrefix(href, "/reports") {
				continue
			}
			// Template actions ({{ .Slug }}) are validated by rendering the
			// landing page against the live router in the black-box tests;
			// the literal action text is not a concrete link.
			if strings.Contains(href, "{{") {
				continue
			}
			if !allowed[href] {
				t.Errorf("%s contains a dead report link: %s", name, href)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §17: report surfaces must never reintroduce IP addresses
// ─────────────────────────────────────────────────────────────────────────────

func TestReportSourcesContainNoIPAddressFields(t *testing.T) {
	forbidden := []string{"ip_address", "ipaddress", "clientip", "remote_addr"}

	sources := []string{
		"../../web/templates/reports.html",
		"../../web/templates/report_detail.html",
		"../../internal/handlers/report_handler.go",
		"../../internal/repository/report_repository.go",
		"../../internal/models/report.go",
		"../../internal/routes/report_routes.go",
	}

	for _, name := range sources {
		raw, err := os.ReadFile(filepath.FromSlash(name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}

		lower := strings.ToLower(string(raw))
		for _, needle := range forbidden {
			if strings.Contains(lower, needle) {
				t.Errorf("%s references %q — audit/report surfaces must stay IP-free", name, needle)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Unauthenticated access: browser is redirected, API gets 401 JSON
// ─────────────────────────────────────────────────────────────────────────────

func TestReportRoutesRejectUnauthenticatedRequests(t *testing.T) {
	router := buildReportTestRouter()

	pageReq := httptest.NewRequest(http.MethodGet, "/reports/donations/page", nil)
	pageReq.Header.Set("Accept", "text/html")
	pageRec := httptest.NewRecorder()
	router.ServeHTTP(pageRec, pageReq)

	if pageRec.Code != http.StatusSeeOther {
		t.Errorf("unauthenticated page: status=%d, want %d", pageRec.Code, http.StatusSeeOther)
	}
	if loc := pageRec.Header().Get("Location"); loc != "/login" {
		t.Errorf("unauthenticated page: Location=%q, want /login", loc)
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/api/reports/donations", nil)
	apiReq.Header.Set("Accept", "application/json")
	apiRec := httptest.NewRecorder()
	router.ServeHTTP(apiRec, apiReq)

	if apiRec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated JSON: status=%d, want %d", apiRec.Code, http.StatusUnauthorized)
	}
	if ct := apiRec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("unauthenticated JSON: Content-Type=%q, want application/json", ct)
	}
}