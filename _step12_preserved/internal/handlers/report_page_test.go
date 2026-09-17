// Package handlers_test hosts the end-to-end report page test: the REAL
// production template set, the REAL route handler and the REAL service and
// repository wired together behind a gin engine, asserting that every report
// page answers HTTP 200 with HTML (never JSON, never a 500). It intentionally
// lives in the external test package so it can import internal/database
// without an import cycle, and it skips when no PostgreSQL is reachable so
// `go test ./...` still succeeds in plain environments.
package handlers_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/i18n"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// reportPageTestDB connects to the configured PostgreSQL database and brings
// it to the canonical state. When no database is reachable the tests are
// skipped so `go test ./...` still succeeds in plain environments.
func reportPageTestDB(t *testing.T) *gin.Engine {
	t.Helper()

	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("report page test skipped (configuration unavailable): %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("report page test skipped (database unavailable): %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("report page test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("report page test role seed failed: %v", err)
	}

	// Production template set — same file list as cmd/server/main.go so the
	// "base" layout resolves every page template it references.
	files := []string{
		"../../web/templates/layouts/base.html",
		"../../web/templates/layouts/header.html",
		"../../web/templates/home.html",
		"../../web/templates/login.html",
		"../../web/templates/forgot_password.html",
		"../../web/templates/verify_reset_otp.html",
		"../../web/templates/reset_password.html",
		"../../web/templates/error.html",
		"../../web/templates/dashboard.html",
		"../../web/templates/beneficiary_dashboard_content.html",
		"../../web/templates/student_dashboard_content.html",
		"../../web/templates/my_aid_content.html",
		"../../web/templates/request_aid_content.html",
		"../../web/templates/my_loans_content.html",
		"../../web/templates/apply_for_loan_content.html",
		"../../web/templates/my_repayments_content.html",
		"../../web/templates/my_care_content.html",
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
		"../../web/templates/student_view.html",
		"../../web/templates/student_edit.html",
		"../../web/templates/donors.html",
		"../../web/templates/donor_view.html",
		"../../web/templates/donor_edit.html",
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
		"../../web/templates/reports.html",
		"../../web/templates/audit_logs.html",
		"../../web/templates/permissions.html",
	}
	tmpl := template.Must(template.ParseFiles(files...))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.SetHTMLTemplate(tmpl)

	// Minimal actor context replicating RequireAuth + LoadPermissions: the
	// handler-level permission checks read this per-request cache.
	router.Use(func(c *gin.Context) {
		c.Set("current_user", &models.User{
			Username: "report-page-test",
			Role:     models.Role{Name: models.RoleAdmin},
		})
		c.Set("permissions", []string{
			"reports.view", "reports.export", "audit_logs.view", "system.alerts.view",
		})
		c.Next()
	})

	reportService := services.NewReportService(repository.NewReportRepository(db))
	reportHandler := handlers.NewReportHandler(reportService)
	router.GET("/reports/dashboard/page", reportHandler.Page)

	return router
}

// reportPageTitles maps each report type to its English page title.
var reportPageTitles = map[string]string{
	"users":          "User Report",
	"beneficiaries":  "Beneficiary Report",
	"account-status": "Account Status Report",
	"students":       "Student Report",
	"donors":         "Donor Report",
	"donations":      "Donation Report",
	"aid-requests":   "Aid Request Report",
	"care":           "Care Provided Report",
	"loans":          "Loan Report",
	"repayments":     "Loan Repayment Report",
	"revenue":        "Revenue Report",
	"audit":          "Audit Activity Report",
	"system-alerts":  "System Alert Report",
}

// TestReportPages_EveryReportTypeLoadsAsHTML performs GET
// /reports/dashboard/page for the report center and every report type and
// asserts HTTP 200 with an HTML document that carries the report's title and
// a detailed table (or the professional empty state when the database has no
// matching records).
func TestReportPages_EveryReportTypeLoadsAsHTML(t *testing.T) {
	router := reportPageTestDB(t)

	cases := []struct {
		report string // query parameter value, "" = report center
		title  string
	}{
		{"", "Report Center"},
		{"users", reportPageTitles["users"]},
		{"beneficiaries", reportPageTitles["beneficiaries"]},
		{"students", reportPageTitles["students"]},
		{"donors", reportPageTitles["donors"]},
		{"donations", reportPageTitles["donations"]},
		{"aid-requests", reportPageTitles["aid-requests"]},
		{"care", reportPageTitles["care"]},
		{"loans", reportPageTitles["loans"]},
		{"repayments", reportPageTitles["repayments"]},
		{"revenue", reportPageTitles["revenue"]},
		{"account-status", reportPageTitles["account-status"]},
		{"audit", reportPageTitles["audit"]},
		{"system-alerts", reportPageTitles["system-alerts"]},
	}

	for _, tc := range cases {
		t.Run("report="+tc.report, func(t *testing.T) {
			url := "/reports/dashboard/page"
			if tc.report != "" {
				url += "?report=" + tc.report
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200; body: %s", url, w.Code, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
				t.Fatalf("GET %s must answer HTML, got Content-Type %q", url, ct)
			}

			body := w.Body.String()
			if !strings.Contains(body, tc.title) {
				t.Errorf("GET %s must render the title %q", url, tc.title)
			}

			if tc.report == "" {
				// Report center: one card per report, no dead links.
				for reportType := range reportPageTitles {
					if !strings.Contains(body, "report="+reportType) {
						t.Errorf("report center must offer the %q report card", reportType)
					}
				}
				return
			}

			// Detail reports: either the detailed table or the empty state —
			// never an empty broken table.
			empty := i18n.T(i18n.DefaultLanguage, "reports.empty")
			if !strings.Contains(body, `<table class="data-table">`) && !strings.Contains(body, empty) {
				t.Errorf("GET %s must render a data table or the empty state %q", url, empty)
			}
		})
	}
}

// TestReportPages_InvalidReportTypeRendersCenterWithBanner proves an unknown
// report type is a safe, user-facing HTML answer (never JSON, never a 404).
func TestReportPages_InvalidReportTypeRendersCenterWithBanner(t *testing.T) {
	router := reportPageTestDB(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/reports/dashboard/page?report=bogus", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET invalid report = %d, want 200 with banner", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, i18n.T(i18n.DefaultLanguage, "reports.unknown")) {
		t.Error("invalid report type must render the translated unknown banner")
	}
	if !strings.Contains(body, "report=users") {
		t.Error("invalid report type must still render the report center cards")
	}
}
