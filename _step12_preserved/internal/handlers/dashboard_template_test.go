package handlers

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/komiga092-glitch/pwams/internal/i18n"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// renderDashboard parses and renders the REAL dashboard content template so a
// regression in its structure — exactly the "unexpected {{end}}" parse panic
// that previously broke GET /dashboard — fails the test suite instead of
// producing a production 500.
func renderDashboard(t *testing.T, data TemplateData) string {
	t.Helper()

	const path = "../../web/templates/dashboard.html"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	tmpl, err := template.New("dashboard_test_root").Parse(string(content))
	if err != nil {
		t.Fatalf("dashboard.html failed to parse: %v (template parse panic)", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Lookup("dashboard_content").Execute(&buf, data); err != nil {
		t.Fatalf("failed to render dashboard_content: %v", err)
	}
	return buf.String()
}

func dashboardTemplateData(superAdmin bool) TemplateData {
	data := TemplateData{"current_language": i18n.DefaultLanguage}
	data["user_permissions"] = map[string]bool{
		"reports.view":       true,
		"system.alerts.view": true,
		"users.view":         true,
	}
	if superAdmin {
		admin := &models.SuperAdminDashboard{
			TotalUsers:     10,
			ActiveUsers:    8,
			ActiveAdmins:   2,
			LockedAccounts: 1,
			Health: &models.SystemHealth{
				Application: models.HealthStatusHealthy,
				Database:    models.HealthStatusHealthy,
				Sync:        models.HealthStatusHealthy,
				Storage:     models.HealthStatusHealthy,
				Email:       models.HealthStatusHealthy,
			},
		}
		data["super_admin"] = admin
		data["stats"] = admin
		return data
	}

	data["stats"] = &models.DashboardStats{
		TotalBeneficiaries:  5,
		TotalStudents:       6,
		TotalDonors:         7,
		ActiveLoans:         3,
		PendingAidRequests:  2,
		RevenueSummary:      decimal.NewFromFloat(12.5),
		UnreadNotifications: 4,
		UsersByRole:         map[string]int64{"Staff": 3},
	}
	return data
}

func TestDashboardTemplate_SuperAdminRenders(t *testing.T) {
	html := renderDashboard(t, dashboardTemplateData(true))
	for _, want := range []string{"10", models.HealthStatusHealthy, "2"} {
		if !strings.Contains(html, want) {
			t.Errorf("super-admin dashboard should render %q", want)
		}
	}
}

func TestDashboardTemplate_AdminRendersStats(t *testing.T) {
	html := renderDashboard(t, dashboardTemplateData(false))
	for _, want := range []string{"5", "Staff", "12.50"} {
		if !strings.Contains(html, want) {
			t.Errorf("admin dashboard should render %q", want)
		}
	}
}

// The dashboard handles an empty dataset (no users by role) via the range's
// {{ else }} branch — this guards the "missing database data" case.
func TestDashboardTemplate_EmptyUsersByRoleRendersEmptyState(t *testing.T) {
	data := dashboardTemplateData(false)
	data["stats"] = &models.DashboardStats{UsersByRole: map[string]int64{}}
	html := renderDashboard(t, data)
	if !strings.Contains(html, i18n.T(i18n.DefaultLanguage, "dashboard.no_role_data")) {
		t.Error("empty UsersByRole should render the no_role_data empty state")
	}
}
