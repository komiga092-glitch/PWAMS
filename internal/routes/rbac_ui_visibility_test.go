package routes_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// Phase 2 regression tests for UI role visibility (§14 Users page role
// options, §20 sidebar / action-button visibility).
//
// UI hiding is NOT the security boundary — the route middleware and the
// service-layer guards are (see internal/services/user_superadmin_guard_test.go
// and the live RBAC matrix). These tests pin the *rendering contract* so the
// menu and the role selectors stay aligned with the backend policy that was
// verified live:
//
//	Manager  -> every operational module, but NOT users/revenue/audit-logs
//	Super Admin -> everything
//	Admin    -> everything except minting Super Admins
//	Staff    -> operational, no users/revenue/audit-logs
//	Donor/Beneficiary/Student/Volunteer -> dashboard/notifications/messages
//
// Run from internal/routes, so templates resolve as ../../web/templates.

func readTemplate(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "web", "templates", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read template %s: %v", path, err)
	}

	return string(raw)
}

// readPageScript reads an external page script from
// web/static/js/pages. Templates must not carry inline JavaScript (strict
// CSP), so behavioral contracts live in these files.
func readPageScript(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "web", "static", "js", "pages", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read page script %s: %v", path, err)
	}

	return string(raw)
}

// navRolesFor returns the data-roles attribute of the sidebar nav link whose
// href is the given path. The brand logo also links to /dashboard without a
// data-roles attribute, so only elements carrying the nav-link class count.
func navRolesFor(t *testing.T, html, href string) string {
	t.Helper()

	needle := `href="` + href + `"`
	searchFrom := 0

	for {
		rel := strings.Index(html[searchFrom:], needle)
		if rel < 0 {
			break
		}

		hrefIndex := searchFrom + rel
		tagStart := strings.LastIndex(html[:hrefIndex], "<a")
		tagEnd := strings.Index(html[hrefIndex:], ">")

		if tagStart >= 0 && tagEnd >= 0 {
			tag := html[tagStart : hrefIndex+tagEnd]

			if strings.Contains(tag, "nav-link") {
				rolesIndex := strings.Index(tag, `data-roles="`)
				if rolesIndex < 0 {
					t.Fatalf("nav link %s has no data-roles attribute", href)
				}

				rest := tag[rolesIndex+len(`data-roles="`):]
				end := strings.Index(rest, `"`)
				if end < 0 {
					t.Fatalf("malformed data-roles attribute on nav link %s", href)
				}

				return rest[:end]
			}
		}

		searchFrom = hrefIndex + len(needle)
	}

	t.Fatalf("sidebar has no nav-link element targeting %s", href)

	return ""
}

func rolesContain(roles, role string) bool {
	for _, candidate := range strings.Split(roles, ",") {
		if strings.TrimSpace(candidate) == role {
			return true
		}
	}

	return false
}

func assertNavAllows(t *testing.T, html, href, role string) {
	t.Helper()

	if roles := navRolesFor(t, html, href); !rolesContain(roles, role) {
		t.Errorf("sidebar %s must be visible to %q, data-roles=%q", href, role, roles)
	}
}

func assertNavDenies(t *testing.T, html, href, role string) {
	t.Helper()

	if roles := navRolesFor(t, html, href); rolesContain(roles, role) {
		t.Errorf("sidebar %s must NOT be visible to %q, data-roles=%q", href, role, roles)
	}
}

// selectBlock returns the <select> markup that starts at marker.
func selectBlock(t *testing.T, html, marker string) string {
	t.Helper()

	start := strings.Index(html, marker)
	if start < 0 {
		t.Fatalf("template has no %s", marker)
	}

	rest := html[start:]
	end := strings.Index(rest, "</select>")
	if end < 0 {
		t.Fatalf("unterminated <select> for %s", marker)
	}

	return rest[:end]
}

func countOption(block, role string) int {
	return strings.Count(block, `<option value="`+role+`"`)
}

var allRoles = []string{
	models.RoleSuperAdmin,
	models.RoleAdmin,
	models.RoleManager,
	models.RoleStaff,
	models.RoleVolunteer,
	models.RoleDonor,
	models.RoleBeneficiary,
	models.RoleStudent,
}

// ─────────────────────────────────────────────────────────────────────────────
// §5 / §20 sidebar visibility
// ─────────────────────────────────────────────────────────────────────────────

func TestSidebar_ManagerSeesEveryOperationalModule(t *testing.T) {
	header := readTemplate(t, filepath.Join("layouts", "header.html"))

	// The exact bug the Phase 2 audit found: Manager received 403 on every
	// NGO operational page and the menu hid them all.
	for _, href := range []string{
		"/dashboard",
		"/persons/page",
		"/care-provided/page",
		"/students/page",
		"/donors/page",
		"/donations/page",
		"/aid-requests/page",
		"/loans/page",
		"/loan-repayments/page",
		"/files/page",
		"/reports/dashboard/page",
	} {
		assertNavAllows(t, header, href, models.RoleManager)
	}
}

func TestSidebar_ManagerCannotSeePlatformModules(t *testing.T) {
	header := readTemplate(t, filepath.Join("layouts", "header.html"))

	for _, href := range []string{"/users/page", "/revenue/page", "/audit-logs/page"} {
		assertNavDenies(t, header, href, models.RoleManager)
	}
}

func TestSidebar_DashboardVisibleToEveryRole(t *testing.T) {
	header := readTemplate(t, filepath.Join("layouts", "header.html"))

	// Login redirects to /dashboard, so every authenticated role must see it.
	for _, role := range allRoles {
		assertNavAllows(t, header, "/dashboard", role)
	}
}

func TestSidebar_PlatformModulesRestrictedToSuperAdminAndAdmin(t *testing.T) {
	header := readTemplate(t, filepath.Join("layouts", "header.html"))

	for _, href := range []string{"/users/page", "/revenue/page", "/audit-logs/page"} {
		for _, role := range []string{models.RoleSuperAdmin, models.RoleAdmin} {
			assertNavAllows(t, header, href, role)
		}
		for _, role := range []string{models.RoleStaff, models.RoleVolunteer, models.RoleDonor, models.RoleBeneficiary, models.RoleStudent} {
			assertNavDenies(t, header, href, role)
		}
	}
}

func TestSidebar_StaffDeniedPlatformModules(t *testing.T) {
	header := readTemplate(t, filepath.Join("layouts", "header.html"))

	for _, href := range []string{"/users/page", "/revenue/page", "/audit-logs/page"} {
		assertNavDenies(t, header, href, models.RoleStaff)
	}
}

func TestOperationalTemplates_DoNotHideManagerFromAllowedActions(t *testing.T) {
	// "Super Admin,Admin,Staff" was the pre-Phase-2 value; it hid every
	// create/edit control from Manager even though the backend allows it.
	const staleValue = `data-roles="Super Admin,Admin,Staff"`

	operational := []string{
		"dashboard.html",
		"persons.html",
		"students.html",
		"donors.html",
		"donations.html",
		"aid_requests.html",
		"care_provided.html",
		"loans.html",
		"loan_repayments.html",
		"files.html",
		"messages.html",
		"notifications.html",
	}

	for _, name := range operational {
		html := readTemplate(t, name)

		if strings.Contains(html, staleValue) {
			t.Errorf("%s still hides controls from Manager: %s", name, staleValue)
		}

		if strings.Contains(html, "data-roles=") && !strings.Contains(html, models.RoleManager) {
			t.Errorf("%s gates controls by role but never mentions %q", name, models.RoleManager)
		}
	}
}

func TestCareProvided_DeleteControlStaysRestrictedToSuperAdminAndAdmin(t *testing.T) {
	html := readTemplate(t, "care_provided.html")

	// Manager gains create/edit (group-level policy) but delete stays
	// Super Admin / Admin only, matching the route middleware. The delete
	// control is emitted by web/static/js/pages/care_provided.js (CSP-safe
	// data-csp-action, never an inline onclick).
	deleteControl := readPageScript(t, "care_provided.js")

	if !strings.Contains(deleteControl, `data-roles="Super Admin,Admin" data-csp-action="care:delete"`) {
		t.Fatal("care-provided delete control must remain gated to Super Admin/Admin")
	}

	if strings.Contains(html, "deleteCareRecord") {
		t.Error("care_provided.html must not inline delete behavior")
	}
}

func TestUsersPage_SuperAdminOptionIsRoleGated(t *testing.T) {
	html := readTemplate(t, "users.html")

	// Offered only to a Super Admin actor...
	if !strings.Contains(html, `<option value="Super Admin" data-roles="Super Admin">`) {
		t.Error(`Super Admin option must be gated with data-roles="Super Admin"`)
	}

	// ...and removed client-side for every other actor (fail closed when the
	// current role cannot be determined). The logic lives in the external
	// page script: templates carry no inline JavaScript (strict CSP).
	usersJS := readPageScript(t, "users.js")
	if !strings.Contains(usersJS, "function restrictRoleOptions()") {
		t.Error("pages/users.js must define restrictRoleOptions()")
	}
	if !strings.Contains(usersJS, `#user-role option[value="Super Admin"]`) {
		t.Error("restrictRoleOptions() must remove the Super Admin option by selector")
	}
	if !strings.Contains(usersJS, "restrictRoleOptions()") {
		t.Error("pages/users.js must call restrictRoleOptions()")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §14 Users page role options
// ─────────────────────────────────────────────────────────────────────────────

func TestUsersPage_AddEditSelectorOffersExactlyTheEightRoles(t *testing.T) {
	html := readTemplate(t, "users.html")
	block := selectBlock(t, html, `id="user-role"`)

	for _, role := range allRoles {
		if got := countOption(block, role); got != 1 {
			t.Errorf("Add/Edit role selector: %d option(s) for %q, want exactly 1", got, role)
		}
	}

	if strings.Contains(block, "Partner") {
		t.Error("Add/Edit role selector must not offer the retired Partner role")
	}
}

func TestUsersPage_FilterListsEveryRole(t *testing.T) {
	html := readTemplate(t, "users.html")
	block := selectBlock(t, html, `name="role" style="max-width: 190px"`)

	for _, role := range allRoles {
		if got := countOption(block, role); got != 1 {
			t.Errorf("role filter: %d option(s) for %q, want exactly 1", got, role)
		}
	}
}

func TestTemplates_DoNotReferenceRetiredPartnerRole(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "web", "templates"))
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}

		if html := readTemplate(t, entry.Name()); strings.Contains(html, "Partner") {
			t.Errorf("%s still references the retired Partner role", entry.Name())
		}
	}
}
