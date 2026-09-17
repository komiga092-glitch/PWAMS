package handlers

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/i18n"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// renderSidebar parses the real sidebar layout and renders it with the given
// template data, so the tests cover exactly what the server sends to clients.
func renderSidebar(t *testing.T, data TemplateData) string {
	t.Helper()

	const headerPath = "../../web/templates/layouts/header.html"
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", headerPath, err)
	}

	// The root must NOT be named "header" — the file itself defines
	// {{ define "header" }}, and a same-named root would collide at
	// parse time ("multiple definition of template \"header\"").
	tmpl, err := template.New("sidebar_test_root").Parse(string(content))
	if err != nil {
		t.Fatalf("failed to parse %s: %v", headerPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Lookup("header").Execute(&buf, data); err != nil {
		t.Fatalf("failed to render header: %v", err)
	}
	return buf.String()
}

// localeData returns a TemplateData pre-populated with the i18n context the
// layout expects from handlers.PageData.
func localeData() TemplateData {
	return TemplateData{"current_language": i18n.DefaultLanguage}
}

func sidebarData(permissions []string) TemplateData {
	user := &models.User{Username: "tester"}
	user.Role = models.Role{Name: models.RoleStaff}
	set := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		set[p] = true
	}
	// The header renders localised labels through the TemplateData methods,
	// exactly as handlers.PageData provides them.
	data := localeData()
	data["current_user"] = user
	data["user_permissions"] = set
	return data
}

func TestSidebar_HidesModulesWithoutPermission(t *testing.T) {
	// Staff user with a typical set but explicitly WITHOUT care.view.
	html := renderSidebar(t, sidebarData([]string{
		"person.view", "donation.view", "users.view", "reports.view", "audit_logs.view",
	}))

	if strings.Contains(html, "/care-provided/page") {
		t.Error("sidebar must NOT render Care Provided for a user without care.view")
	}
	if strings.Contains(html, "Care Provided") {
		t.Error("sidebar must not leak the Care Provided module name")
	}
	if !strings.Contains(html, "/persons/page") {
		t.Error("sidebar must render Care Seekers for a user with person.view")
	}
}

func TestSidebar_ShowsModulesWithPermission(t *testing.T) {
	html := renderSidebar(t, sidebarData([]string{
		"person.view", "care.view", "student.view", "donation.view",
		"aid.view", "loan.view", "repayment.view", "revenue.view",
		"notification.view", "message.view", "file.view", "users.view",
		"reports.view", "audit_logs.view", "permission_management.view",
	}))

	for _, want := range []string{
		"/persons/page", "/care-provided/page", "/students/page",
		"/donations/page", "/aid-requests/page", "/loans/page",
		"/loan-repayments/page", "/revenue/page", "/notifications/page",
		"/messages/page", "/files/page", "/users/page",
		"/reports/dashboard/page", "/audit-logs/page", "/permissions/page",
		"/dashboard", "/profile",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("sidebar should contain %s for a fully-privileged user", want)
		}
	}
}

func TestSidebar_AuditLogsHiddenWithoutPermission(t *testing.T) {
	html := renderSidebar(t, sidebarData([]string{"donor.view"}))

	if strings.Contains(html, "/audit-logs/page") {
		t.Error("sidebar must NOT render Audit Logs without audit_logs.view")
	}
	if strings.Contains(html, "Audit Logs") {
		t.Error("sidebar must not leak the Audit Logs module name")
	}
	if strings.Contains(html, "/users/page") {
		t.Error("sidebar must NOT render Users without users.view")
	}
	if strings.Contains(html, "/reports/dashboard/page") {
		t.Error("sidebar must NOT render Reports without reports.view")
	}
}

// If the user has permission for NO management module the whole Management
// section header must disappear (no empty dropdown/section).
func TestSidebar_HidesEmptyManagementSection(t *testing.T) {
	html := renderSidebar(t, sidebarData([]string{"care.view", "notification.view"}))

	if strings.Contains(html, ">Operations<") {
		t.Error("Management section header must be hidden when no management module is visible")
	}
	if !strings.Contains(html, "/care-provided/page") {
		t.Error("Care Provided must remain visible with care.view")
	}
}

func TestSidebar_ShowsOperationsSectionWhenAnyChildVisible(t *testing.T) {
	html := renderSidebar(t, sidebarData([]string{"repayment.view"}))

	if !strings.Contains(html, ">Operations<") {
		t.Error("Operations section header must be shown when at least one operational child is visible")
	}
	if !strings.Contains(html, "/loan-repayments/page") {
		t.Error("Repayments must remain visible with repayment.view")
	}
}

// Anonymous visitors must not see any navigation items (no information leak).
func TestSidebar_AnonymousSeesNoNavigation(t *testing.T) {
	data := localeData()
	data["current_user"] = nil
	data["user_permissions"] = map[string]bool{}
	html := renderSidebar(t, data)

	// The brand logo links to /dashboard and is always rendered; every actual
	// navigation item must be absent.
	for _, want := range []string{
		`class="nav-link"`, "/profile", "/logout", "/donors/page",
		"/users/page", "/audit-logs/page",
	} {
		if strings.Contains(html, want) {
			t.Errorf("anonymous sidebar must not contain %q", want)
		}
	}
}

// beneficiarySidebarData builds sidebar data for a self-service role.
func beneficiarySidebarData(role string, permissions []string) TemplateData {
	user := &models.User{Username: "tester"}
	user.Role = models.Role{Name: role}
	set := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		set[p] = true
	}
	data := localeData()
	data["current_user"] = user
	data["user_permissions"] = set
	return data
}

// A Beneficiary with the self-service permission set must see the self-service
// links and must NOT see administrative modules.
func TestSidebar_BeneficiarySeesSelfServiceOnly(t *testing.T) {
	html := renderSidebar(t, beneficiarySidebarData(models.RoleBeneficiary, []string{
		"aid.view_own", "aid.request", "loan.view_own", "loan.apply",
		"repayment.view_own", "care.view_own", "notification.view",
	}))

	for _, want := range []string{
		"/my/dashboard", "/my/aid", "/my/aid/request", "/my/loans",
		"/my/loans/apply", "/my/repayments", "/my/care",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("beneficiary sidebar should contain %s", want)
		}
	}

	for _, forbidden := range []string{
		"/users/page", "/audit-logs/page", "/reports/dashboard/page",
		"/donors/page", "/donations/page", "/persons/page", "/students/page",
		"/aid-requests/page", "/loans/page", "/loan-repayments/page",
		"/revenue/page", "/permissions/page",
	} {
		if strings.Contains(html, forbidden) {
			t.Errorf("beneficiary sidebar must NOT contain %s", forbidden)
		}
	}
}

// A Student must see the self-service links (except My Care, which requires
// care.view_own) and must NOT see administrative modules.
func TestSidebar_StudentSeesSelfServiceOnly(t *testing.T) {
	html := renderSidebar(t, beneficiarySidebarData(models.RoleStudent, []string{
		"aid.view_own", "aid.request", "loan.view_own", "loan.apply",
		"repayment.view_own", "notification.view",
	}))

	for _, want := range []string{
		"/my/dashboard", "/my/aid", "/my/aid/request", "/my/loans",
		"/my/loans/apply", "/my/repayments",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("student sidebar should contain %s", want)
		}
	}

	if strings.Contains(html, "/my/care") {
		t.Error("student sidebar must NOT contain My Care without care.view_own")
	}

	for _, forbidden := range []string{
		"/users/page", "/audit-logs/page", "/reports/dashboard/page",
		"/persons/page", "/students/page", "/aid-requests/page",
		"/loans/page", "/loan-repayments/page",
	} {
		if strings.Contains(html, forbidden) {
			t.Errorf("student sidebar must NOT contain %s", forbidden)
		}
	}
}

// An Admin (admin.view held, system.settings.view NOT held) must see the
// Administration entries it owns — Access Management and Audit Logs — next to
// the full normal NGO module navigation. Account management is consolidated
// under the single Users module (/users/page); there are no per-role sidebar
// entries for admins/managers/staff/volunteers. System Settings stays
// exclusive to the system owner.
func TestSidebar_AdminSeesAdministrationPlusOperationalModules(t *testing.T) {
	data := beneficiarySidebarData(models.RoleAdmin, []string{
		"admin.view", "permission_management.view", "audit_logs.view",
		"person.view", "care.view", "student.view",
		"donor.view", "donation.view", "aid.view", "loan.view",
		"users.view", "reports.view", "file.view",
	})
	html := renderSidebar(t, data)

	for _, want := range []string{
		"/permissions/page", "/audit-logs/page",
		"Administration",
		"/persons/page", "/care-provided/page", "/users/page",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("admin sidebar should contain %s", want)
		}
	}

	if strings.Contains(html, "/system-settings/page") {
		t.Error("admin sidebar must NOT render System Settings without system.settings.view")
	}
}

// The Super Admin (full permission set) must see EVERY module section: the
// Administration entries AND all operational NGO modules.
func TestSidebar_SuperAdminSeesEveryModuleSection(t *testing.T) {
	data := beneficiarySidebarData(models.RoleSuperAdmin, []string{
		"admin.view", "permission_management.view", "audit_logs.view",
		"system.settings.view", "person.view", "care.view", "student.view",
		"donation.view", "aid.view", "loan.view",
		"repayment.view", "revenue.view", "users.view", "reports.view",
		"notification.view", "message.view", "file.view",
	})
	html := renderSidebar(t, data)

	for _, want := range []string{
		"/permissions/page", "/audit-logs/page",
		"/system-settings/page", "Administration",
		"/persons/page", "/care-provided/page",
		"/students/page",
		"/donations/page", "/aid-requests/page",
		"/loans/page", "/loan-repayments/page", "/revenue/page",
		"/users/page", "Operations",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("super-admin sidebar should contain %s", want)
		}
	}
}
