package components

import (
	"context"
	"strings"
	"testing"
)

func renderTemplSidebar(t *testing.T, authenticated bool, permissions map[string]bool) string {
	t.Helper()

	var buf strings.Builder
	if err := Sidebar(authenticated, permissions).Render(context.Background(), &buf); err != nil {
		t.Fatalf("failed to render Sidebar: %v", err)
	}
	return buf.String()
}

// The templ sidebar mirrors the html/template sidebar: modules without the
// .view permission must not be rendered at all.
func TestSidebarTempl_HidesModulesWithoutPermission(t *testing.T) {
	html := renderTemplSidebar(t, true, map[string]bool{"donor.view": true})

	if strings.Contains(html, "/care-provided/page") {
		t.Error("templ sidebar must NOT render Care Provided without care.view")
	}
	if !strings.Contains(html, "/donors/page") {
		t.Error("templ sidebar must render Donors with donor.view")
	}
	if strings.Contains(html, "/audit-logs/page") {
		t.Error("templ sidebar must NOT render Audit Logs without audit_logs.view")
	}
	// donor.view is a Management child, so the section header must be shown.
	if !strings.Contains(html, ">Management<") {
		t.Error("Management section header must be shown when a management module is visible")
	}
	// Without admin.view the Administration section must not appear at all.
	if strings.Contains(html, "/admins/page") {
		t.Error("templ sidebar must NOT render Admin Management without admin.view")
	}
}

// If the user has permission for NO management module the whole Management
// section header must disappear (no empty section).
func TestSidebarTempl_HidesEmptyManagementSection(t *testing.T) {
	html := renderTemplSidebar(t, true, map[string]bool{"care.view": true})

	if strings.Contains(html, ">Management<") {
		t.Error("Management section header must be hidden when no management module is visible")
	}
	if !strings.Contains(html, "/care-provided/page") {
		t.Error("Care Provided must remain visible with care.view")
	}
}

func TestSidebarTempl_ShowsModulesWithPermission(t *testing.T) {
	html := renderTemplSidebar(t, true, map[string]bool{
		"person.view": true, "care.view": true, "student.view": true,
		"donor.view": true, "donation.view": true, "aid.view": true,
		"loan.view": true, "repayment.view": true, "revenue.view": true,
		"notification.view": true, "message.view": true, "file.view": true,
		"users.view": true, "reports.view": true, "audit_logs.view": true,
		"permission_management.view": true,
	})

	for _, want := range []string{
		"/persons/page", "/care-provided/page", "/students/page", "/donors/page",
		"/donations/page", "/aid-requests/page", "/loans/page",
		"/loan-repayments/page", "/revenue/page", "/users/page",
		"/reports/dashboard/page", "/audit-logs/page", "/permissions/page",
		">Management<",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("templ sidebar should contain %s", want)
		}
	}
}

func TestSidebarTempl_AnonymousRendersNoNavItems(t *testing.T) {
	html := renderTemplSidebar(t, false, nil)

	if strings.Contains(html, `class="nav-link"`) {
		t.Error("anonymous templ sidebar must not render navigation items")
	}
	if strings.Contains(html, "/donors/page") || strings.Contains(html, "/users/page") {
		t.Error("anonymous templ sidebar must not leak module links")
	}
}

// A user holding admin.view (held by the Super Admin and Admin roles) gets
// the Administration navigation. Per the final role model the Administration
// section is ADDITIVE: the Super Admin also sees every operational NGO module
// for which it holds the .view permission — the old either/or fork that hid
// all Management modules from admin.view holders was an over-restriction and
// is gone.
func TestSidebarTempl_SuperAdminSeesAdministrationAndOperationalModules(t *testing.T) {
	html := renderTemplSidebar(t, true, map[string]bool{
		"admin.view": true, "permission_management.view": true,
		"audit_logs.view": true, "system.settings.view": true,
		"person.view": true, "care.view": true, "partner.view": true,
		"staff.view": true, "volunteer.view": true, "student.view": true,
		"donor.view": true, "donation.view": true, "aid.view": true,
		"loan.view": true, "repayment.view": true, "revenue.view": true,
		"users.view": true, "reports.view": true,
		"notification.view": true, "message.view": true, "file.view": true,
	})

	for _, want := range []string{
		// Administration entries (additive section).
		"/admins/page", "/permissions/page", "/audit-logs/page",
		"/system-settings/page", ">Administration<", "Admin Management",
		"Access Management", "Audit Logs", "System Settings",
		// Operational NGO modules — must NOT be hidden from admin.view
		// holders any more.
		"/persons/page", "/care-provided/page", "/partners/page",
		"/staff/page", "/volunteers/page", "/students/page",
		"/donors/page", "/donations/page", "/aid-requests/page",
		"/loans/page", "/loan-repayments/page", "/revenue/page",
		"/users/page", ">Management<",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("super-admin templ sidebar should contain %s", want)
		}
	}
}

// An Admin (admin.view held, system.settings.view NOT held) must see the
// Administration entries it owns — Admin Management, Access Management and
// Audit Logs — next to the full normal NGO module navigation, while the
// System Settings entry stays exclusive to the system owner.
func TestSidebarTempl_AdminSeesAdministrationAndNormalModules(t *testing.T) {
	html := renderTemplSidebar(t, true, map[string]bool{
		"admin.view": true, "permission_management.view": true,
		"audit_logs.view": true,
		"person.view":     true, "care.view": true, "partner.view": true,
		"staff.view": true, "volunteer.view": true, "donor.view": true,
		"users.view": true, "reports.view": true,
	})

	for _, want := range []string{
		"/admins/page", "/permissions/page", "/audit-logs/page",
		">Administration<", "Admin Management",
		"/persons/page", "/care-provided/page", "/partners/page",
		"/staff/page", "/volunteers/page", "/donors/page", "/users/page",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("admin templ sidebar should contain %s", want)
		}
	}

	if strings.Contains(html, "/system-settings/page") {
		t.Error("admin templ sidebar must NOT render System Settings without system.settings.view")
	}
}

// Partner / Staff / Volunteer links must render exactly when the matching
// .view permission is held (spec section 25).
func TestSidebarTempl_ShowsPartnersStaffVolunteers(t *testing.T) {
	html := renderTemplSidebar(t, true, map[string]bool{
		"partner.view": true, "staff.view": true, "volunteer.view": true,
	})

	for _, want := range []string{"/partners/page", "/staff/page", "/volunteers/page"} {
		if !strings.Contains(html, want) {
			t.Errorf("templ sidebar should contain %s", want)
		}
	}

	if !strings.Contains(html, ">Management<") {
		t.Error("Management section header must be shown when Partners/Staff/Volunteers are visible")
	}
}

// Without the .view permission the partner/staff/volunteer links must not
// render at all.
func TestSidebarTempl_HidesPartnersStaffVolunteersWithoutPermission(t *testing.T) {
	html := renderTemplSidebar(t, true, map[string]bool{"donor.view": true})

	for _, forbidden := range []string{"/partners/page", "/staff/page", "/volunteers/page"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("templ sidebar must NOT contain %s without the matching .view", forbidden)
		}
	}
}
