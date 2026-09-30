package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/i18n"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// ─────────────────────────────────────────────────────────────────────────────
// Report route registry — the ONE authoritative map of report areas.
//
// Every report has exactly one HTML page route (/reports/{slug}/page), one
// JSON API route (/api/reports/{slug}) and, where data supports it, one PDF
// route (/reports/{slug}/pdf). Routes are registered from this registry (see
// internal/routes/report_routes.go), report cards link to it, and the route
// registry tests iterate over it — so a dead link cannot survive review.
// ─────────────────────────────────────────────────────────────────────────────

// ReportDef describes one report area.
type ReportDef struct {
	// Slug is the URL segment: /reports/{slug}/page, /reports/{slug}/pdf,
	// /api/reports/{slug}.
	Slug string
	// Template is the page content block rendered inside base.html.
	Template string
	// PlatformOnly restricts the report to Super Admin and Admin (users,
	// revenue, account status, audit, system alerts). Operational NGO
	// reports additionally allow Manager and Staff, matching the Phase 2
	// module authorization.
	PlatformOnly bool
	// HasDetail marks reports with a row-level, filterable, paginated
	// detail table in addition to the summary cards.
	HasDetail bool
}

// ReportRegistry returns every implemented report area in stable order.
func ReportRegistry() []ReportDef {
	return []ReportDef{
		{Slug: "dashboard", Template: "report_dashboard_content"},
		{Slug: "users", Template: "report_users_content", PlatformOnly: true, HasDetail: true},
		{Slug: "persons", Template: "report_persons_content", HasDetail: true},
		{Slug: "students", Template: "report_students_content", HasDetail: true},
		{Slug: "donors", Template: "report_donors_content", HasDetail: true},
		{Slug: "donations", Template: "report_donations_content", HasDetail: true},
		{Slug: "aid-requests", Template: "report_aid_requests_content", HasDetail: true},
		{Slug: "care-provided", Template: "report_care_provided_content", HasDetail: true},
		{Slug: "loans", Template: "report_loans_content", HasDetail: true},
		{Slug: "loan-repayments", Template: "report_loan_repayments_content", HasDetail: true},
		{Slug: "revenue", Template: "report_revenue_content", PlatformOnly: true, HasDetail: true},
		{Slug: "account-status", Template: "report_account_status_content", PlatformOnly: true},
		{Slug: "audit-log", Template: "report_audit_log_content", PlatformOnly: true, HasDetail: true},
		{Slug: "system-alerts", Template: "report_system_alerts_content", PlatformOnly: true},
	}
}

var reportTitleKeys = map[string]string{
	"dashboard":       "reports.dashboard_report",
	"users":           "reports.users_report",
	"persons":         "reports.beneficiaries_report",
	"students":        "reports.students_report",
	"donors":          "reports.donors_report",
	"donations":       "reports.donations_report",
	"aid-requests":    "reports.aid_requests_report",
	"care-provided":   "reports.care_provided_report",
	"loans":           "reports.loans_report",
	"loan-repayments": "reports.loan_repayments_report",
	"revenue":         "reports.revenue_report",
	"account-status":  "reports.account_status_report",
	"audit-log":       "reports.audit_log_report",
	"system-alerts":   "reports.system_alerts_report",
}

// ReportHandler serves every report surface (HTML page, JSON API, PDF).
type ReportHandler struct {
	reportService    *services.ReportService
	reportPDFService *services.ReportPDFService
}

func NewReportHandler(
	reportService *services.ReportService,
	reportPDFService *services.ReportPDFService,
) *ReportHandler {
	return &ReportHandler{
		reportService:    reportService,
		reportPDFService: reportPDFService,
	}
}

// PageFor returns the HTML route handler for a report area.
func (h *ReportHandler) PageFor(def ReportDef) gin.HandlerFunc {
	return func(c *gin.Context) { h.servePage(c, def) }
}

// JSONFor returns the JSON API handler for a report area.
func (h *ReportHandler) JSONFor(def ReportDef) gin.HandlerFunc {
	return func(c *gin.Context) { h.serveJSON(c, def) }
}

// PDFFor returns the PDF download handler for a report area.
func (h *ReportHandler) PDFFor(def ReportDef) gin.HandlerFunc {
	return func(c *gin.Context) { h.servePDF(c, def) }
}

// ─────────────────────────────────────────────────────────────────────────────
// Reports landing page (/reports/page)
// ─────────────────────────────────────────────────────────────────────────────

type reportLandingCard struct {
	Slug           string
	TitleKey       string
	DescriptionKey string
	PlatformOnly   bool
	Count          *int64
	CountLabelKey  string
}

type reportLandingCategory struct {
	NameKey string
	Cards   []reportLandingCard
}

// LandingPage renders the reports landing page grouped by category, with a
// small live summary number on each card where a cheap aggregate exists.
// Platform-only cards carry data-roles so the UI hides them for non-admin
// roles, but the backend authorization on each report route remains the
// actual security boundary.
func (h *ReportHandler) LandingPage(c *gin.Context) {
	donations, donErr := h.reportService.GetDonationReport()
	aidRequests, aidErr := h.reportService.GetAidRequestReport()
	loans, loanErr := h.reportService.GetLoanReport()
	students, stuErr := h.reportService.GetStudentReport()
	donors, donorErr := h.reportService.GetDonorReport()
	care, careErr := h.reportService.GetCareProvidedReport()

	if donErr != nil || aidErr != nil || loanErr != nil || stuErr != nil || donorErr != nil || careErr != nil {
		renderReportErrorPage(c, http.StatusInternalServerError, "Unable to load reports")
		return
	}

	intPtr := func(v int64) *int64 { return &v }

	categories := []reportLandingCategory{
		{
			NameKey: "reports.category.overview",
			Cards: []reportLandingCard{{
				Slug:           "dashboard",
				TitleKey:       "reports.dashboard_report",
				DescriptionKey: "reports.dashboard_report_subtitle",
			}},
		},
		{
			NameKey: "reports.category.people",
			Cards: []reportLandingCard{
				{
					Slug:           "persons",
					TitleKey:       "reports.beneficiaries_report",
					DescriptionKey: "reports.beneficiaries_report_subtitle",
				},
				{
					Slug:           "students",
					TitleKey:       "reports.students_report",
					DescriptionKey: "reports.students_report_subtitle",
					Count:          intPtr(students.ActiveStudents),
					CountLabelKey:  "reports.active_students",
				},
				{
					Slug:           "donors",
					TitleKey:       "reports.donors_report",
					DescriptionKey: "reports.donors_report_subtitle",
					Count:          intPtr(donors.ActiveDonors),
					CountLabelKey:  "reports.active_donors",
				},
				{
					Slug:           "users",
					TitleKey:       "reports.users_report",
					DescriptionKey: "reports.users_report_subtitle",
					PlatformOnly:   true,
				},
			},
		},
		{
			NameKey: "reports.category.welfare",
			Cards: []reportLandingCard{
				{
					Slug:           "aid-requests",
					TitleKey:       "reports.aid_requests_report",
					DescriptionKey: "reports.aid_requests_report_subtitle",
					Count:          intPtr(aidRequests.PendingRequests),
					CountLabelKey:  "reports.pending_requests",
				},
				{
					Slug:           "care-provided",
					TitleKey:       "reports.care_provided_report",
					DescriptionKey: "reports.care_provided_report_subtitle",
					Count:          intPtr(care.CompletedRecords),
					CountLabelKey:  "reports.completed",
				},
			},
		},
		{
			NameKey: "reports.category.finance",
			Cards: []reportLandingCard{
				{
					Slug:           "donations",
					TitleKey:       "reports.donations_report",
					DescriptionKey: "reports.donations_report_subtitle",
					Count:          intPtr(donations.TotalDonations),
					CountLabelKey:  "reports.total_donations",
				},
				{
					Slug:           "loans",
					TitleKey:       "reports.loans_report",
					DescriptionKey: "reports.loans_report_subtitle",
					Count:          intPtr(loans.ActiveLoans),
					CountLabelKey:  "reports.active_loans",
				},
				{
					Slug:           "loan-repayments",
					TitleKey:       "reports.loan_repayments_report",
					DescriptionKey: "reports.loan_repayments_report_subtitle",
					Count:          intPtr(loans.CompletedLoans),
					CountLabelKey:  "reports.completed_loans",
				},
				{
					Slug:           "revenue",
					TitleKey:       "reports.revenue_report",
					DescriptionKey: "reports.revenue_report_subtitle",
					PlatformOnly:   true,
				},
			},
		},
		{
			NameKey: "reports.category.administration",
			Cards: []reportLandingCard{
				{
					Slug:           "account-status",
					TitleKey:       "reports.account_status_report",
					DescriptionKey: "reports.account_status_subtitle",
					PlatformOnly:   true,
				},
				{
					Slug:           "audit-log",
					TitleKey:       "reports.audit_log_report",
					DescriptionKey: "reports.audit_log_subtitle",
					PlatformOnly:   true,
				},
				{
					Slug:           "system-alerts",
					TitleKey:       "reports.system_alerts_report",
					DescriptionKey: "reports.system_alerts_subtitle",
					PlatformOnly:   true,
				},
			},
		},
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "reports_content",
		"title":         i18n.T(langString(c), "reports.title"),
		"categories":    categories,
	}))
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared serving logic (HTML / JSON / PDF)
// ─────────────────────────────────────────────────────────────────────────────

// servePage renders the HTML report page: summary cards always, plus the
// filter + paginated detail table for detail reports. It never returns JSON.
func (h *ReportHandler) servePage(c *gin.Context, def ReportDef) {
	filter := parseReportFilter(c)

	summary, err := h.loadSummary(def, filter)
	if err != nil {
		renderReportErrorPage(c, http.StatusInternalServerError, "Unable to load this report")
		return
	}

	data := gin.H{
		"page_template": def.Template,
		"title":         i18n.T(langString(c), reportTitleKeys[def.Slug]),
		"report":        summary,
		"report_slug":   def.Slug,
	}

	if def.HasDetail {
		rows, total, err := h.loadRows(def.Slug, filter)
		if err != nil {
			renderReportErrorPage(c, http.StatusInternalServerError, "Unable to load report rows")
			return
		}

		// Shared table projection: HTML tables and PDF exports use the
		// identical formatting path, so dates/currency/status never drift.
		_, tableHeaders, tableRows := reportPDFContent(langString(c), def.Slug, summary, rows)

		data["table_headers"] = tableHeaders
		data["table_rows"] = tableRows
		data["pagination"] = buildReportPagination(c, total, filter)
		data["filters"] = filter
		data["status_options"] = reportStatusOptions(def.Slug)
		data["status_label_key"] = reportStatusLabelKey(def.Slug)
	}

	c.HTML(http.StatusOK, "base", PageData(c, data))
}

// serveJSON returns the structured JSON report:
// {"success":true,"summary":{...},"data":[...],"pagination":{...}} — "data"
// and "pagination" only on detail reports. It never returns HTML.
func (h *ReportHandler) serveJSON(c *gin.Context, def ReportDef) {
	filter := parseReportFilter(c)

	summary, err := h.loadSummary(def, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to load this report",
		})
		return
	}

	payload := gin.H{
		"success": true,
		"message": "Report retrieved successfully",
		"summary": summary,
	}

	if def.HasDetail {
		rows, total, err := h.loadRows(def.Slug, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to load report rows",
			})
			return
		}

		payload["data"] = rows
		payload["pagination"] = buildReportPagination(c, total, filter)
	}

	c.JSON(http.StatusOK, payload)
}

// servePDF streams the report PDF with the correct content type and a useful
// filename. Failures are JSON with a non-200 status — never an HTML body.
func (h *ReportHandler) servePDF(c *gin.Context, def ReportDef) {
	filter := parseReportFilter(c)

	summary, err := h.loadSummary(def, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to load this report",
		})
		return
	}

	var rows any
	if def.HasDetail {
		rows, _, err = h.loadRows(def.Slug, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to load report rows",
			})
			return
		}
	}

	pdfBytes, err := h.buildReportPDF(langString(c), def, summary, rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to generate report PDF",
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(
		"attachment; filename=%q",
		services.ReportPDFFilename(def.Slug),
	))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func langString(c *gin.Context) string {
	if lang, ok := c.Get("lang"); ok {
		if s, ok := lang.(string); ok && s != "" {
			return s
		}
	}
	return "en"
}

// parseReportFilter binds and clamps the shared report filter. Every value
// reaches the repository as a bound parameter — never as interpolated SQL.
//
// Malformed input is dropped rather than propagated: an unparsable date or an
// unknown status/priority is discarded here so a hand-typed query string can
// never reach the repository as an error (which would surface as a 500).
func parseReportFilter(c *gin.Context) models.ReportFilter {
	var filter models.ReportFilter
	_ = c.ShouldBindQuery(&filter)

	filter.From = normalizeReportDate(filter.From)
	filter.To = normalizeReportDate(filter.To)

	filter.Status = strings.TrimSpace(filter.Status)
	filter.Priority = strings.TrimSpace(filter.Priority)
	filter.Type = strings.TrimSpace(filter.Type)
	filter.Role = strings.TrimSpace(filter.Role)
	filter.Grade = strings.TrimSpace(filter.Grade)
	filter.Query = strings.TrimSpace(filter.Query)

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	return filter
}

// normalizeReportDate keeps only a well-formed YYYY-MM-DD value; anything else
// (including oversized or non-date input) becomes the empty filter.
func normalizeReportDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return ""
	}

	return parsed.Format("2006-01-02")
}

// buildReportPagination computes the pagination descriptor for detail
// reports, including Previous/Next URLs that preserve the active filters.
func buildReportPagination(
	c *gin.Context,
	total int64,
	filter models.ReportFilter,
) *models.ReportPagination {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}

	pageURL := func(page int) string {
		query := c.Request.URL.Query()
		query.Set("page", strconv.Itoa(page))
		return c.Request.URL.Path + "?" + query.Encode()
	}

	pagination := &models.ReportPagination{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalItems: total,
		TotalPages: totalPages,
		HasPrev:    filter.Page > 1,
		HasNext:    totalPages > 0 && filter.Page < totalPages,
	}

	if pagination.HasPrev {
		pagination.PrevURL = pageURL(filter.Page - 1)
	}
	if pagination.HasNext {
		pagination.NextURL = pageURL(filter.Page + 1)
	}

	return pagination
}

// renderReportErrorPage renders the standalone error page (the same
// error.html the RBAC middleware uses). Raw internal errors are never
// surfaced to the user.
func renderReportErrorPage(c *gin.Context, code int, message string) {
	c.HTML(code, "error.html", gin.H{
		"title":   "Error",
		"heading": "Error",
		"message": message,
		"Lang":    langString(c),
	})
}

// loadSummary dispatches to the summary query for a report area. Reports with a
// detail table are served through a filter-scoped service view, so their summary
// cards count exactly the rows the table shows (a report must reflect only the
// data inside the specified range/filter). Reports without a detail table have
// no filterable rows, so they always use the shared, unfiltered service.
func (h *ReportHandler) loadSummary(def ReportDef, filter models.ReportFilter) (any, error) {
	reportService := h.reportService

	if def.HasDetail {
		scoped, err := h.reportService.WithFilter(filter)
		if err != nil {
			return nil, err
		}
		reportService = scoped
	}

	switch def.Slug {
	case "dashboard":
		return reportService.GetDashboardReport()
	case "users":
		return reportService.GetUsersReport()
	case "persons":
		return reportService.GetPersonReport()
	case "students":
		return reportService.GetStudentReport()
	case "donors":
		return reportService.GetDonorReport()
	case "donations":
		return reportService.GetDonationReport()
	case "aid-requests":
		return reportService.GetAidRequestReport()
	case "care-provided":
		return reportService.GetCareProvidedReport()
	case "loans":
		return reportService.GetLoanReport()
	case "loan-repayments":
		return reportService.GetLoanRepaymentReport()
	case "revenue":
		return reportService.GetRevenueReport()
	case "account-status":
		return reportService.GetAccountStatusReport()
	case "audit-log":
		return reportService.GetAuditLogReport()
	case "system-alerts":
		return reportService.GetSystemAlertReport()
	}

	return nil, fmt.Errorf("unknown report: %s", def.Slug)
}

// loadRows dispatches to the paginated detail query for a report slug.
func (h *ReportHandler) loadRows(slug string, filter models.ReportFilter) (any, int64, error) {
	switch slug {
	case "users":
		return h.reportService.GetUserReportRows(filter)
	case "persons":
		return h.reportService.GetPersonReportRows(filter)
	case "students":
		return h.reportService.GetStudentReportRows(filter)
	case "donors":
		return h.reportService.GetDonorReportRows(filter)
	case "donations":
		return h.reportService.GetDonationReportRows(filter)
	case "aid-requests":
		return h.reportService.GetAidRequestReportRows(filter)
	case "care-provided":
		return h.reportService.GetCareProvidedReportRows(filter)
	case "loans":
		return h.reportService.GetLoanReportRows(filter)
	case "loan-repayments":
		return h.reportService.GetLoanRepaymentReportRows(filter)
	case "revenue":
		return h.reportService.GetRevenueReportRows(filter)
	case "audit-log":
		return h.reportService.GetAuditLogReportRows(filter)
	}

	return nil, 0, nil
}

// reportStatusOptions returns the status/category filter options for a
// report's filter form. Only values that exist as model constants are
// offered — no invented business states. Revenue is filtered by category.
func reportStatusOptions(slug string) []string {
	switch slug {
	case "users":
		return []string{models.UserStatusActive, models.UserStatusDisabled, models.UserStatusLocked}
	case "persons":
		return []string{models.PersonStatusActive, models.PersonStatusInactive, models.PersonStatusPending}
	case "students":
		return []string{models.StudentStatusActive, models.StudentStatusInactive, models.StudentStatusPending}
	case "donors":
		return []string{models.DonorStatusActive, models.DonorStatusInactive, models.DonorStatusPending}
	case "donations":
		return []string{models.DonationStatusPending, models.DonationStatusConfirmed, models.DonationStatusCancelled}
	case "aid-requests":
		return []string{
			models.AidStatusPending, models.AidStatusUnderReview, models.AidStatusApproved,
			models.AidStatusRejected, models.AidStatusCompleted, models.AidStatusCancelled,
		}
	case "care-provided":
		return []string{models.CareProvidedStatusPending, models.CareProvidedStatusCompleted, models.CareProvidedStatusCancelled}
	case "loans":
		return []string{
			models.LoanStatusPending, models.LoanStatusApproved, models.LoanStatusActive,
			models.LoanStatusCompleted, models.LoanStatusRejected, models.LoanStatusCancelled,
		}
	case "loan-repayments":
		return []string{models.RepaymentStatusPending, models.RepaymentStatusPaid, models.RepaymentStatusOverdue, models.RepaymentStatusCancelled}
	case "revenue":
		return []string{
			models.RevenueCategoryDonations, models.RevenueCategoryLoanRepayments,
			models.RevenueCategoryGrants, models.RevenueCategoryAdministrativeExpense,
			models.RevenueCategoryWelfareExpense,
		}
	}

	return nil
}

// reportStatusLabelKey selects the i18n label for the status/category filter.
func reportStatusLabelKey(slug string) string {
	if slug == "revenue" {
		return "revenue.category"
	}
	return "common.status"
}

// ─────────────────────────────────────────────────────────────────────────────
// PDF assembly
// ─────────────────────────────────────────────────────────────────────────────

// buildReportPDF renders one report as a PDF document. Aggregate-only areas
// produce a summary sheet; detail areas add the filter-matched table (first
// page, max 100 rows — a documented export limitation).
func (h *ReportHandler) buildReportPDF(
	lang string,
	def ReportDef,
	summary any,
	rows any,
) ([]byte, error) {
	summaryRows, headers, table := reportPDFContent(lang, def.Slug, summary, rows)

	return h.reportPDFService.RenderTableReport(&services.PDFTableReport{
		Title:       i18n.T(lang, reportTitleKeys[def.Slug]),
		Subtitle:    i18n.T(lang, "reports.generated_from_live"),
		SummaryRows: summaryRows,
		Headers:     headers,
		Rows:        table,
	})
}

func pdfDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}

func pdfMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func pdfNum(v int64) string {
	return fmt.Sprintf("%d", v)
}

// pdfText trims and caps free-text cells so long descriptions cannot break
// the PDF table layout.
func pdfText(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

// reportPDFContent converts a typed report summary + row slice into the
// generic PDF table input. All user-facing labels go through i18n with the
// report viewer's language.
func reportPDFContent(
	lang string,
	slug string,
	summary any,
	rows any,
) (summaryRows [][2]string, headers []string, table [][]string) {
	T := func(key string) string { return i18n.T(lang, key) }

	switch slug {
	case "dashboard":
		d := summary.(*models.DashboardReport)
		return [][2]string{
			{T("reports.total_users"), pdfNum(d.TotalUsers)},
			{T("reports.total_persons"), pdfNum(d.TotalPersons)},
			{T("reports.total_students"), pdfNum(d.TotalStudents)},
			{T("reports.total_donors"), pdfNum(d.TotalDonors)},
			{T("reports.total_donations"), pdfNum(d.TotalDonations)},
			{T("reports.total_aid_requests"), pdfNum(d.TotalAidRequests)},
			{T("reports.total_care_provided"), pdfNum(d.TotalCareProvided)},
		}, nil, nil

	case "users":
		u := summary.(*models.UsersReport)
		summaryRows = [][2]string{
			{T("reports.total_users"), pdfNum(u.TotalUsers)},
			{T("reports.active_users"), pdfNum(u.ActiveUsers)},
			{T("reports.disabled_users"), pdfNum(u.DisabledUsers)},
			{T("reports.locked_users"), pdfNum(u.LockedUsers)},
			{T("reports.new_last_30_days"), pdfNum(u.NewLast30Days)},
		}
		headers = []string{
			T("users.username"), T("users.email"), T("users.role"),
			T("common.status"), T("audit.created_at"),
		}
		if userRows, ok := rows.([]models.UserReportRow); ok {
			for _, r := range userRows {
				table = append(table, []string{
					pdfText(r.Username, 40), pdfText(r.Email, 50), pdfText(r.RoleName, 20),
					pdfText(r.Status, 12), pdfDate(r.CreatedAt),
				})
			}
		}

	case "persons":
		p := summary.(*models.PersonReport)
		summaryRows = [][2]string{
			{T("reports.total_persons"), pdfNum(p.TotalPersons)},
			{T("reports.active_persons"), pdfNum(p.ActivePersons)},
			{T("reports.inactive"), pdfNum(p.InactivePersons)},
			{T("common.pending"), pdfNum(p.PendingPersons)},
		}
		headers = []string{
			T("persons.full_name"), T("persons.nic_passport"), T("persons.gender"),
			T("common.status"), T("donations.date"),
		}
		if personRows, ok := rows.([]models.PersonReportRow); ok {
			for _, r := range personRows {
				table = append(table, []string{
					pdfText(r.FullName, 40), pdfText(r.NICPassport, 20), pdfText(r.Gender, 10),
					pdfText(r.Status, 12), pdfDate(r.CreatedAt),
				})
			}
		}

	case "students":
		s := summary.(*models.StudentReport)
		summaryRows = [][2]string{
			{T("reports.total_students"), pdfNum(s.TotalStudents)},
			{T("reports.active_students"), pdfNum(s.ActiveStudents)},
			{T("reports.inactive"), pdfNum(s.InactiveStudents)},
			{T("common.pending"), pdfNum(s.PendingStudents)},
			{T("reports.recent_students"), pdfNum(s.RecentStudents)},
		}
		headers = []string{
			T("students.full_name"), T("students.student_code"), T("students.school"),
			T("students.grade"), T("students.academic_year"), T("common.status"),
		}
		if studentRows, ok := rows.([]models.StudentReportRow); ok {
			for _, r := range studentRows {
				table = append(table, []string{
					pdfText(r.FullName, 35), pdfText(r.StudentCode, 15), pdfText(r.SchoolName, 35),
					pdfText(r.Grade, 10), pdfNum(int64(r.AcademicYear)), pdfText(r.Status, 12),
				})
			}
		}

	case "donors":
		d := summary.(*models.DonorReport)
		summaryRows = [][2]string{
			{T("reports.total_donors"), pdfNum(d.TotalDonors)},
			{T("reports.active_donors"), pdfNum(d.ActiveDonors)},
			{T("reports.inactive"), pdfNum(d.InactiveDonors)},
			{T("reports.total_donations"), pdfNum(d.TotalDonations)},
			{T("reports.total_donation_amount"), pdfMoney(d.TotalDonationAmount)},
		}
		headers = []string{
			T("donors.name"), T("donors.type"), T("persons.phone"),
			T("common.status"), T("donations.date"),
		}
		if donorRows, ok := rows.([]models.DonorReportRow); ok {
			for _, r := range donorRows {
				table = append(table, []string{
					pdfText(r.Name, 40), pdfText(r.DonorType, 15), pdfText(r.Phone, 15),
					pdfText(r.Status, 12), pdfDate(r.CreatedAt),
				})
			}
		}

	case "donations":
		d := summary.(*models.DonationReport)
		summaryRows = [][2]string{
			{T("reports.total_donations"), pdfNum(d.TotalDonations)},
			{T("reports.total_amount"), pdfMoney(d.TotalAmount)},
			{T("common.pending"), pdfNum(d.PendingDonations)},
			{T("reports.confirmed"), pdfNum(d.ConfirmedDonations)},
			{T("reports.cancelled"), pdfNum(d.CancelledDonations)},
		}
		headers = []string{
			T("reports.donor"), T("reports.type"), T("reports.item"),
			T("reports.amount"), T("donations.date"), T("common.status"),
		}
		if donationRows, ok := rows.([]models.DonationReportRow); ok {
			for _, r := range donationRows {
				table = append(table, []string{
					pdfText(r.DonorName, 30), pdfText(r.DonationType, 12), pdfText(r.ItemName, 25),
					r.Amount.StringFixed(2), pdfDate(r.DonationDate), pdfText(r.Status, 12),
				})
			}
		}

	case "aid-requests":
		a := summary.(*models.AidRequestReport)
		summaryRows = [][2]string{
			{T("reports.total_requests"), pdfNum(a.TotalRequests)},
			{T("common.pending"), pdfNum(a.PendingRequests)},
			{T("reports.under_review"), pdfNum(a.UnderReviewCount)},
			{T("reports.approved"), pdfNum(a.ApprovedRequests)},
			{T("reports.rejected"), pdfNum(a.RejectedRequests)},
			{T("reports.completed"), pdfNum(a.CompletedRequests)},
		}
		headers = []string{
			T("aid_requests.title_label"), T("aid_requests.beneficiary"), T("reports.type"),
			T("aid_requests.priority"), T("reports.amount"), T("donations.date"), T("common.status"),
		}
		if aidRows, ok := rows.([]models.AidRequestReportRow); ok {
			for _, r := range aidRows {
				table = append(table, []string{
					pdfText(r.Title, 30), pdfText(r.PersonName, 25), pdfText(r.AidType, 12),
					pdfText(r.Priority, 10), r.RequestedAmount.StringFixed(2),
					pdfDate(r.RequestDate), pdfText(r.Status, 12),
				})
			}
		}

	case "care-provided":
		c := summary.(*models.CareProvidedReport)
		summaryRows = [][2]string{
			{T("reports.total_care_provided"), pdfNum(c.TotalRecords)},
			{T("reports.completed"), pdfNum(c.CompletedRecords)},
			{T("common.pending"), pdfNum(c.PendingRecords)},
			{T("reports.cancelled"), pdfNum(c.CancelledRecords)},
			{T("reports.total_amount"), pdfMoney(c.TotalAmount)},
		}
		headers = []string{
			T("care.person"), T("care.care_type"), T("care.provided_by"),
			T("reports.amount"), T("care.date"), T("common.status"),
		}
		if careRows, ok := rows.([]models.CareProvidedReportRow); ok {
			for _, r := range careRows {
				table = append(table, []string{
					pdfText(r.PersonName, 30), pdfText(r.CareType, 15), pdfText(r.ProvidedBy, 25),
					pdfMoney(r.Amount), pdfDate(r.ProvidedAt), pdfText(r.Status, 12),
				})
			}
		}

	case "loans":
		l := summary.(*models.LoanReport)
		summaryRows = [][2]string{
			{T("reports.total_loans"), pdfNum(l.TotalLoans)},
			{T("reports.active_loans"), pdfNum(l.ActiveLoans)},
			{T("common.pending"), pdfNum(l.PendingLoans)},
			{T("reports.completed_loans"), pdfNum(l.CompletedLoans)},
			{T("reports.total_amount"), pdfMoney(l.TotalDisbursed)},
			{T("reports.outstanding"), pdfMoney(l.OutstandingAmount)},
		}
		headers = []string{
			T("loans.person"), T("reports.amount"), T("loans.interest"),
			T("loans.duration_months"), T("common.status"), T("donations.date"),
		}
		if loanRows, ok := rows.([]models.LoanReportRow); ok {
			for _, r := range loanRows {
				table = append(table, []string{
					pdfText(r.PersonName, 30), r.LoanAmount.StringFixed(2),
					r.InterestRate.StringFixed(2) + "%", pdfNum(int64(r.PeriodMonths)),
					pdfText(r.Status, 12), pdfDate(r.CreatedAt),
				})
			}
		}

	case "loan-repayments":
		rp := summary.(*models.LoanRepaymentReport)
		summaryRows = [][2]string{
			{T("reports.total_repayments"), pdfNum(rp.TotalRepayments)},
			{T("reports.paid"), pdfNum(rp.PaidRepayments)},
			{T("common.pending"), pdfNum(rp.PendingRepayments)},
			{T("reports.overdue"), pdfNum(rp.OverdueRepayments)},
			{T("reports.paid_amount"), pdfMoney(rp.TotalPaidAmount)},
			{T("reports.outstanding"), pdfMoney(rp.OutstandingAmount)},
		}
		headers = []string{
			T("loans.person"), T("loan_repayments.installment"), T("loan_repayments.due_date"),
			T("reports.amount"), T("reports.paid_amount"), T("common.status"),
		}
		if repaymentRows, ok := rows.([]models.LoanRepaymentReportRow); ok {
			for _, r := range repaymentRows {
				table = append(table, []string{
					pdfText(r.PersonName, 30), pdfNum(int64(r.InstallmentNumber)),
					pdfDate(r.DueDate), r.Amount.StringFixed(2),
					r.PaidAmount.StringFixed(2), pdfText(r.Status, 12),
				})
			}
		}

	case "revenue":
		rv := summary.(*models.RevenueReport)
		summaryRows = [][2]string{
			{T("reports.total_income"), pdfMoney(rv.TotalIncome)},
			{T("reports.total_expenses"), pdfMoney(rv.TotalExpenses)},
			{T("reports.net"), pdfMoney(rv.Net)},
		}
		headers = []string{
			T("revenue.type"), T("revenue.category"), T("reports.amount"),
			T("revenue.date"), T("revenue.description"),
		}
		if revenueRows, ok := rows.([]models.RevenueReportRow); ok {
			for _, r := range revenueRows {
				table = append(table, []string{
					pdfText(r.RecordType, 10), pdfText(r.Category, 20),
					r.Amount.StringFixed(2), pdfDate(r.RecordDate), pdfText(r.Description, 40),
				})
			}
		}

	case "account-status":
		ac := summary.(*models.AccountStatusReport)
		summaryRows = [][2]string{
			{T("reports.total_accounts"), pdfNum(ac.TotalUsers)},
			{T("reports.active_users"), pdfNum(ac.ActiveUsers)},
			{T("reports.disabled_users"), pdfNum(ac.DisabledUsers)},
			{T("reports.locked_users"), pdfNum(ac.LockedUsers)},
			{T("reports.locked_now"), pdfNum(ac.LockedNow)},
			{T("reports.never_logged_in"), pdfNum(ac.NeverLoggedIn)},
		}

	case "audit-log":
		al := summary.(*models.AuditLogReport)
		summaryRows = [][2]string{
			{T("reports.total_entries"), pdfNum(al.TotalEntries)},
		}
		// Audit PDF columns are intentionally free of IP addresses: the
		// audit trail stores none and reports must never expose any.
		headers = []string{
			T("audit.created_at"), T("users.username"), T("audit.action"),
			T("audit.entity"), T("audit.details"), T("audit_log.request_id"),
		}
		if auditRows, ok := rows.([]models.AuditLogReportRow); ok {
			for _, r := range auditRows {
				table = append(table, []string{
					pdfDate(r.CreatedAt), pdfText(r.Username, 25), pdfText(r.Action, 25),
					pdfText(r.Entity, 20), pdfText(r.Details, 50), pdfText(r.RequestID, 20),
				})
			}
		}

	case "system-alerts":
		sa := summary.(*models.SystemAlertReport)
		summaryRows = [][2]string{
			{T("reports.total_alerts"), pdfNum(sa.TotalAlerts)},
			{T("reports.failed_logins"), pdfNum(sa.FailedLogins)},
			{T("reports.locked_accounts"), pdfNum(sa.LockedAccounts)},
			{T("reports.denied_access"), pdfNum(sa.DeniedAccess)},
			{T("reports.recent_7_days"), pdfNum(sa.RecentAlerts)},
		}
	}

	return summaryRows, headers, table
}
