package handlers

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/i18n"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// ── Report type mapping ─────────────────────────────────────────────────

// TestReportKey_MapsEverySupportedReportType pins the 13 supported report
// types: every URL type must resolve to a non-empty i18n key so the page
// always renders a translated title, and unknown types must not.
func TestReportKey_MapsEverySupportedReportType(t *testing.T) {
	supported := []string{
		"users", "beneficiaries", "account-status", "students", "donors",
		"donations", "aid-requests", "care", "loans", "repayments",
		"revenue", "audit", "system-alerts",
	}
	for _, reportType := range supported {
		if reportKey(reportType) == "" {
			t.Errorf("report type %q must map to a non-empty i18n key", reportType)
		}
	}

	for _, invalid := range []string{"", "nope", "USERS_CSV", "admin"} {
		if reportKey(invalid) != "" {
			t.Errorf("report type %q must not resolve to a key", invalid)
		}
	}
}

func TestIsValidReportType(t *testing.T) {
	if !isValidReportType("system-alerts") {
		t.Error("system-alerts must be a valid report type")
	}
	if isValidReportType("bogus") {
		t.Error("bogus must be an invalid report type")
	}
}

// TestReportTypeParam_MapsTypeFilters verifies that the per-report
// type/category filter parameter name is exposed for every report that
// supports one.
func TestReportTypeParam_MapsTypeFilters(t *testing.T) {
	cases := map[string]string{
		"donations":    "donation_type",
		"aid-requests": "aid_type",
		"care":         "care_type",
		"revenue":      "category",
	}
	for reportType, want := range cases {
		if got := reportTypeParam(reportType); got != want {
			t.Errorf("reportTypeParam(%q) = %q, want %q", reportType, got, want)
		}
	}
	if got := reportTypeParam("users"); got != "" {
		t.Errorf("reportTypeParam(users) = %q, want empty", got)
	}
}

// ── Filter preservation during pagination ───────────────────────────────

// TestQueryStringWithoutPage_PreservesFiltersAndDropsPage proves pagination
// links keep every active filter (search/status/date range) while stripping
// only the page parameter.
func TestQueryStringWithoutPage_PreservesFiltersAndDropsPage(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet,
		"/reports/dashboard/page?page=2&search=abc&status=Active&date_from=2026-01-01&date_to=2026-09-05", nil)

	qs := queryStringWithoutPage(c)

	if strings.Contains(qs, "page=2") {
		t.Errorf("page parameter must be stripped, got %q", qs)
	}
	for _, must := range []string{"search=abc", "status=Active", "date_from=2026-01-01", "date_to=2026-09-05"} {
		if !strings.Contains(qs, must) {
			t.Errorf("query string %q must preserve %q", qs, must)
		}
	}
}

// ── Permission checks (defence-in-depth inside the handler) ─────────────

// TestHasPermission_FailsClosedWithoutCachedSet proves the handler-level
// permission check denies when no permission cache is present (the
// LoadPermissions middleware was bypassed) — never trusting frontend hiding.
func TestHasPermission_FailsClosedWithoutCachedSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	if hasPermission(c, "reports.view") {
		t.Fatal("missing permission cache must fail closed")
	}
}

// TestHasPermission_CaseInsensitiveCacheLookup verifies the cached set is
// matched case-insensitively and that a wrong cache type is denied.
func TestHasPermission_CaseInsensitiveCacheLookup(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{"  Reports.View  "})
	if !hasPermission(c, "reports.view") {
		t.Error("cache lookup must be case-insensitive and trim whitespace")
	}

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Set("permissions", "reports.view")
	if hasPermission(c2, "reports.view") {
		t.Error("a non-list cache value must be denied")
	}
}

// ── Pagination extraction ───────────────────────────────────────────────

// TestReportPagination_ExtractsEveryResultType proves the pagination block is
// available for every report result type (the shared pagination partial reads
// it) and that unknown payloads are rejected.
func TestReportPagination_ExtractsEveryResultType(t *testing.T) {
	pag := models.ReportPagination{Page: 2, PageSize: 20, TotalItems: 45, TotalPages: 3}
	results := []interface{}{
		&models.UserReportResult{Pagination: pag},
		&models.PersonReportResult{Pagination: pag},
		&models.StudentReportResult{Pagination: pag},
		&models.DonorReportResult{Pagination: pag},
		&models.DonationReportResult{Pagination: pag},
		&models.AidRequestReportResult{Pagination: pag},
		&models.CareProvidedReportResult{Pagination: pag},
		&models.LoanReportResult{Pagination: pag},
		&models.LoanRepaymentReportResult{Pagination: pag},
		&models.RevenueReportResult{Pagination: pag},
		&models.AccountStatusReportResult{Pagination: pag},
		&models.AuditActivityReportResult{Pagination: pag},
		&models.SystemAlertReportResult{Pagination: pag},
	}
	for i, result := range results {
		got, ok := reportPagination(result)
		if !ok {
			t.Fatalf("result %d (%T) must expose a pagination block", i, result)
		}
		if got != pag {
			t.Fatalf("result %d (%T) pagination mismatch: %+v", i, result, got)
		}
	}

	if _, ok := reportPagination("not a report"); ok {
		t.Error("unknown payload must not report a pagination block")
	}
	if _, ok := reportPagination(nil); ok {
		t.Error("nil payload must not report a pagination block")
	}
}

// ── Template rendering (real reports.html) ──────────────────────────────

// renderReportsTemplate parses and renders the REAL reports content template
// so a structural regression — exactly the previously broken pagination
// reference that 500-ed every non-empty report page — fails the suite instead
// of producing production 500s.
func renderReportsTemplate(t *testing.T, data TemplateData) string {
	t.Helper()

	const path = "../../web/templates/reports.html"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	tmpl, err := template.New("reports_test_root").Parse(string(content))
	if err != nil {
		t.Fatalf("reports.html failed to parse: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Lookup("reports_content").Execute(&buf, data); err != nil {
		t.Fatalf("failed to render reports_content: %v", err)
	}
	return buf.String()
}

var (
	reportFixtureTime = time.Date(2026, 9, 5, 10, 30, 0, 0, time.UTC)
	reportFixturePag  = models.ReportPagination{Page: 2, PageSize: 20, TotalItems: 45, TotalPages: 3}
)

// fixtureReport builds one representative result per report type. withRows
// controls whether the detail table carries records (empty-state test uses
// withRows=false).
func fixtureReport(t *testing.T, reportType string, withRows bool) interface{} {
	t.Helper()
	has := withRows
	switch reportType {
	case "users":
		r := &models.UserReportResult{Total: 45, Active: 40, Disabled: 4, Locked: 1, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.UserReportRow{{
				ID: "u-1", Username: "alice", Email: "alice@example.com",
				Role: models.RoleAdmin, Status: models.UserStatusActive,
				LastLogin: &reportFixtureTime, CreatedAt: reportFixtureTime,
			}}
		}
		return r
	case "beneficiaries":
		r := &models.PersonReportResult{Total: 7, Active: 5, Inactive: 2, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.PersonReportRow{{
				ID: "p-1", FullName: "Ben A", NICPassport: "NIC-1", Phone: "0771234567",
				Status: models.PersonStatusActive, CreatedAt: reportFixtureTime,
			}}
		}
		return r
	case "students":
		r := &models.StudentReportResult{Total: 9, Active: 7, Inactive: 2, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.StudentReportRow{{
				ID: "s-1", FullName: "Stu Dent", School: "CPS", Grade: "5",
				Status: models.StudentStatusActive, CreatedAt: reportFixtureTime, UpdatedAt: reportFixtureTime,
			}}
		}
		return r
	case "donors":
		r := &models.DonorReportResult{Total: 12, Active: 10, Inactive: 2, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.DonorReportRow{{
				ID: "d-1", Name: "Donor One", Email: "d@example.com", Phone: "011",
				Status: models.DonorStatusActive, TotalDonations: 3, CreatedAt: reportFixtureTime,
			}}
		}
		return r
	case "donations":
		r := &models.DonationReportResult{TotalDonations: 12, TotalAmount: 1000, Completed: 8, Pending: 4, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.DonationReportRow{{
				ID: "do-1", ReferenceNo: "DN-1", DonorName: "Donor One", DonationType: "Cash",
				Amount: 100, Currency: "LKR", DonationDate: reportFixtureTime,
				Status: models.DonationStatusPending, RecordedBy: "admin", CreatedAt: reportFixtureTime,
			}}
		}
		return r
	default:
		return fixtureReportPart2(t, reportType, withRows)
	}
}

// fixtureReportPart2 carries the remaining report fixtures (aid-requests …
// revenue) so each chunk stays small.
func fixtureReportPart2(t *testing.T, reportType string, withRows bool) interface{} {
	t.Helper()
	has := withRows
	switch reportType {
	case "aid-requests":
		r := &models.AidRequestReportResult{Total: 6, Pending: 2, Approved: 3, Rejected: 1, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.AidRequestReportRow{{
				ID: "ar-1", Title: "School kit", BeneficiaryName: "Ben A", AidType: "Financial",
				RequestedAmount: 500, RequestDate: reportFixtureTime,
				Status: models.AidStatusPending, ApprovedBy: "reviewer",
			}}
		}
		return r
	case "care":
		r := &models.CareProvidedReportResult{Total: 4, Active: 1, Completed: 3, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.CareProvidedReportRow{{
				ID: "c-1", PersonName: "Ben A", CareType: "Medical",
				Status: models.CareProvidedStatusCompleted, ProvidedDate: reportFixtureTime,
				ProvidedBy: "Dr Who", CreatedAt: reportFixtureTime,
			}}
		}
		return r
	case "loans":
		r := &models.LoanReportResult{TotalLoans: 5, Active: 2, Completed: 2, TotalPrincipal: 900, TotalRepaid: 300, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.LoanReportRow{{
				ID: "l-1", ReferenceNo: "LN-1", BorrowerName: "Ben A",
				Principal: 100, PaidAmount: 50, Outstanding: 50,
				Status: models.LoanStatusActive, IssueDate: reportFixtureTime,
				DueDate: &reportFixtureTime, LastRepayment: &reportFixtureTime,
			}}
		}
		return r
	case "repayments":
		r := &models.LoanRepaymentReportResult{Total: 9, TotalAmount: 400, Collected: 350, Overdue: 1, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.LoanRepaymentReportRow{{
				ID: "r-1", PaymentRef: "RP-1", LoanID: "l-1", Installment: 2,
				BorrowerName: "Ben A", Amount: 100, PaidAmount: 100,
				Status: models.RepaymentStatusPaid, DueDate: reportFixtureTime, PaidAt: &reportFixtureTime,
			}}
		}
		return r
	case "revenue":
		r := &models.RevenueReportResult{TotalIncome: 800, TotalExpenses: 200, Net: 600, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.RevenueReportRow{{
				ID: "rev-1", ReferenceNo: "RV-1", RecordType: "income", Category: "Donation",
				Amount: 50, RecordDate: reportFixtureTime, RecordedBy: "admin", CreatedAt: reportFixtureTime,
			}}
		}
		return r
	default:
		return fixtureReportPart3(t, reportType, withRows)
	}
}

// fixtureReportPart3 carries the account-status, audit and system-alert
// fixtures.
func fixtureReportPart3(t *testing.T, reportType string, withRows bool) interface{} {
	t.Helper()
	has := withRows
	switch reportType {
	case "account-status":
		r := &models.AccountStatusReportResult{Total: 45, Active: 40, Disabled: 4, Locked: 1, Pagination: reportFixturePag}
		r.Distribution = []models.AccountStatusDistributionRow{{Role: models.RoleAdmin, Active: 2, Total: 2}}
		if has {
			r.Rows = []models.AccountStatusReportRow{{
				ID: "u-1", Username: "alice", Email: "alice@example.com",
				Role: models.RoleAdmin, Status: models.UserStatusActive,
				LastLogin: &reportFixtureTime, CreatedAt: reportFixtureTime,
			}}
		}
		return r
	case "audit":
		r := &models.AuditActivityReportResult{Total: 30, Creates: 10, Updates: 15, Deletes: 5, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.AuditActivityReportRow{{
				ID: "a-1", Timestamp: reportFixtureTime, Username: "alice",
				Action: "create", Entity: "donation", EntityID: "do-1", Details: "created donation",
			}}
		}
		return r
	case "system-alerts":
		r := &models.SystemAlertReportResult{Total: 8, Critical: 1, Errors: 3, Warnings: 4, Pagination: reportFixturePag}
		if has {
			r.Rows = []models.SystemAlertReportRow{{
				ID: "sa-1", Severity: "ERROR", Category: "database", AlertCode: "DB_DOWN",
				Title: "Database unreachable", Status: "unresolved", Occurrences: 3,
				FirstOccurred: reportFixtureTime, LastOccurred: reportFixtureTime,
			}}
		}
		return r
	default:
		t.Fatalf("no fixture for report type %q", reportType)
		return nil
	}
}

// reportTemplateData builds the template data the Page handler would pass for
// the given report type (page 2 of 3, 45 total records).
func reportTemplateData(t *testing.T, reportType string, withRows bool) TemplateData {
	data := TemplateData{
		"current_language": i18n.DefaultLanguage,
		"report_type":      reportType,
		"title_key":        "reports." + reportKey(reportType) + ".title",
		"subtitle_key":     "reports." + reportKey(reportType) + ".subtitle",
		"can_export":       true,
		"query_string":     template.URL("report=" + reportType + "&search=abc"),
		"filter_roles":     models.AllRoles(),
		"f_search":         "abc",
		"f_role":           "",
		"f_status":         "",
		"f_date_from":      "",
		"f_date_to":        "",
		"f_type":           "",
		"f_type_param":     reportTypeParam(reportType),
		"f_action":         "",
		"f_severity":       "",
		"report":           fixtureReport(t, reportType, withRows),
	}
	if pag, ok := reportPagination(data["report"]); ok {
		data["Pagination"] = pag
		data["total_items"] = pag.TotalItems
		data["total_pages"] = pag.TotalPages
		data["has_prev"] = pag.Page > 1
		data["prev_page"] = pag.Page - 1
		data["has_next"] = pag.Page < pag.TotalPages
		data["next_page"] = pag.Page + 1
		data["showing_from"] = int64((pag.Page-1)*pag.PageSize) + 1
		to := int64(pag.Page * pag.PageSize)
		if to > pag.TotalItems {
			to = pag.TotalItems
		}
		data["showing_to"] = to
	}

	statuses := []string{
		models.UserStatusActive, models.UserStatusDisabled, models.UserStatusLocked,
		models.PersonStatusInactive, models.DonationStatusPending, models.DonationStatusConfirmed,
		models.AidStatusPending, models.AidStatusApproved, models.AidStatusRejected,
		models.AidStatusCompleted, models.CareProvidedStatusCompleted,
		models.LoanStatusActive, models.LoanStatusCompleted, models.RepaymentStatusPaid,
		models.RepaymentStatusOverdue,
	}
	labels := make(map[string]string, len(statuses))
	for _, s := range statuses {
		labels[s] = i18n.T(i18n.DefaultLanguage, "status."+strings.ToLower(s))
	}
	data["status_labels"] = labels
	return data
}

// reportViewLinks maps each report type to the detail link its rows render so
// the "View Details" behaviour is pinned per report.
var reportViewLinks = map[string]string{
	"users":          `/users/u-1`,
	"beneficiaries":  `/persons/p-1`,
	"students":       `/students/s-1`,
	"donors":         `/donors/d-1`,
	"donations":      `/donations/do-1`,
	"aid-requests":   `/aid-requests/ar-1`,
	"care":           `/care-provided/c-1`,
	"loans":          `/loans/l-1`,
	"repayments":     `/loans/l-1`,
	"revenue":        `/revenue/rev-1`,
	"account-status": `/users/u-1`,
}

// TestReportsTemplate_RendersEveryReportWithData renders the real template
// for all 13 report types with records and asserts: translated title, a
// detailed data table with rows, per-row view links and the "Showing X–Y of
// Z" pagination summary with preserved filters.
func TestReportsTemplate_RendersEveryReportWithData(t *testing.T) {
	supported := []string{
		"users", "beneficiaries", "account-status", "students", "donors",
		"donations", "aid-requests", "care", "loans", "repayments",
		"revenue", "audit", "system-alerts",
	}

	for _, reportType := range supported {
		t.Run(reportType, func(t *testing.T) {
			data := reportTemplateData(t, reportType, true)
			html := renderReportsTemplate(t, data)

			title := i18n.T(i18n.DefaultLanguage, data["title_key"].(string))
			if !strings.Contains(html, title) {
				t.Errorf("report page must render its title %q", title)
			}
			if !strings.Contains(html, `<table class="data-table">`) {
				t.Error("report page must render a detailed data table")
			}
			if strings.Contains(html, i18n.T(i18n.DefaultLanguage, "reports.empty")) {
				t.Error("report has rows but rendered the empty state")
			}
			if link, ok := reportViewLinks[reportType]; ok {
				if !strings.Contains(html, link) {
					t.Errorf("report rows must carry the view-details link %q", link)
				}
			}

			// Pagination: page 2 of 3 over 45 records → "Showing 21–40 of 45".
			showing := i18n.T(i18n.DefaultLanguage, "reports.showing")
			of := i18n.T(i18n.DefaultLanguage, "reports.of")
			if !strings.Contains(html, showing+" 21–40 "+of+" 45") {
				t.Errorf("pagination must render the record range %q", showing+" 21–40 "+of+" 45")
			}
			pageLabel := i18n.T(i18n.DefaultLanguage, "reports.page_label")
			if !strings.Contains(html, pageLabel+" 2 / 3") {
				t.Errorf("pagination must render the page indicator %q", pageLabel+" 2 / 3")
			}
			// Pagination links keep the active filters (html/template escapes
			// attribute ampersands, so match the components individually).
			if !strings.Contains(html, "page=3") {
				t.Error("pagination must render a next-page link")
			}
			if !strings.Contains(html, "search=abc") {
				t.Error("pagination links must preserve the active search filter")
			}
		})
	}
}

// TestReportsTemplate_EmptyStateRendersInsteadOfBrokenTable proves a report
// with no records shows the professional empty state (never an empty table).
func TestReportsTemplate_EmptyStateRendersInsteadOfBrokenTable(t *testing.T) {
	data := reportTemplateData(t, "users", false)
	html := renderReportsTemplate(t, data)

	empty := i18n.T(i18n.DefaultLanguage, "reports.empty")
	if !strings.Contains(html, empty) {
		t.Errorf("empty report must render the empty state %q", empty)
	}
}

// TestReportsTemplate_CenterListsAllReports proves the report center renders
// the KPI summary plus one card per report — and that the privileged audit /
// system-alert cards only render for actors with the matching permissions.
func TestReportsTemplate_CenterListsAllReports(t *testing.T) {
	data := TemplateData{
		"current_language": i18n.DefaultLanguage,
		"dashboard": &models.DashboardReport{
			TotalUsers: 10, TotalPersons: 20, TotalStudents: 30, TotalDonors: 40,
			TotalDonations: 50, TotalAidRequests: 60, TotalCareProvided: 70,
		},
		"can_view_audit":  true,
		"can_view_alerts": true,
	}
	html := renderReportsTemplate(t, data)

	for _, reportType := range []string{
		"users", "beneficiaries", "students", "donors", "donations",
		"aid-requests", "care", "loans", "repayments", "revenue",
		"account-status", "audit", "system-alerts",
	} {
		if !strings.Contains(html, "report="+reportType) {
			t.Errorf("report center must offer the %q report card", reportType)
		}
	}
	if !strings.Contains(html, i18n.T(i18n.DefaultLanguage, "reports.available")) {
		t.Error("report center must render the Available Reports heading")
	}
	if !strings.Contains(html, "70") {
		t.Error("report center must render the KPI summary cards")
	}

	// Without the privileged permissions the cards must be absent.
	data["can_view_audit"] = false
	data["can_view_alerts"] = false
	restricted := renderReportsTemplate(t, data)
	if strings.Contains(restricted, "report=audit") || strings.Contains(restricted, "report=system-alerts") {
		t.Error("audit/system-alert cards must not render without the matching permission")
	}
}

// TestReportsTemplate_UnknownReportShowsBanner proves an invalid report type
// answers with the report center plus a safe translated banner (no JSON, no
// raw error, no 404 dead end).
func TestReportsTemplate_UnknownReportShowsBanner(t *testing.T) {
	data := TemplateData{
		"current_language": i18n.DefaultLanguage,
		"unknown_report":   true,
	}
	html := renderReportsTemplate(t, data)

	if !strings.Contains(html, i18n.T(i18n.DefaultLanguage, "reports.unknown")) {
		t.Error("unknown report type must render the translated banner")
	}
	if !strings.Contains(html, "report=users") {
		t.Error("unknown report type must still render the report center cards")
	}
}
