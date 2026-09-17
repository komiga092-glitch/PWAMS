package handlers

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// renderUsersContent parses the real Users page template and renders the
// users_content block with the given template data, so the tests cover
// exactly what the server sends to clients.
func renderUsersContent(t *testing.T, data TemplateData) string {
	t.Helper()

	const usersPath = "../../web/templates/users.html"
	content, err := os.ReadFile(usersPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", usersPath, err)
	}

	tmpl, err := template.New("users_content_test_root").Parse(string(content))
	if err != nil {
		t.Fatalf("failed to parse %s: %v", usersPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Lookup("users_content").Execute(&buf, data); err != nil {
		t.Fatalf("failed to render users_content: %v", err)
	}
	return buf.String()
}

// usersPageData builds the template data the Users page needs, deriving the
// role choices from the backend hierarchy exactly like handlers.PageData.
func usersPageData(actorRole string) TemplateData {
	set := map[string]bool{
		"users.view":           true,
		"users.create":         true,
		"users.edit":           true,
		"users.reset_password": true,
		"users.delete":         true,
	}

	data := TemplateData{
		"current_language": "en",
		"user_permissions": set,
		"assignable_roles": services.AssignableRolesFor(actorRole),
		"filter_roles":     models.AllRoles(),
		"status_labels": map[string]string{
			models.UserStatusActive:   "Active",
			models.UserStatusDisabled: "Disabled",
			models.UserStatusLocked:   "Locked",
		},
	}

	user := &models.User{Username: "actor"}
	user.Role = models.Role{Name: actorRole}
	data["current_user"] = user

	return data
}

// addUserRoleSelector isolates the Add-User modal role selector from the
// rendered page. The read-side role FILTER renders the same <option> markup
// and legitimately lists every role, so assertions on role availability must
// be scoped to this selector only.
func addUserRoleSelector(t *testing.T, html string) string {
	t.Helper()

	const marker = `id="user-role"`
	start := strings.Index(html, marker)
	if start < 0 {
		t.Fatal(`Add-User role selector (id="user-role") not found in rendered page`)
	}

	end := strings.Index(html[start:], "</select>")
	if end < 0 {
		t.Fatal("Add-User role selector is not closed")
	}

	return html[start : start+end]
}

// TestUsersPage_AdminRoleSelectorShowsAllNormalRoles proves the Add User role
// selector for an Admin actor offers Admin, Manager (Partner role), Staff,
// Volunteer, Donor, Beneficiary and Student — and never Super Admin.
func TestUsersPage_AdminRoleSelectorShowsAllNormalRoles(t *testing.T) {
	selector := addUserRoleSelector(t, renderUsersContent(t, usersPageData(models.RoleAdmin)))

	for _, want := range []string{
		`<option value="Admin">Admin</option>`,
		`<option value="Partner">Manager</option>`, // internal Partner renders as Manager
		`<option value="Staff">Staff</option>`,
		`<option value="Volunteer">Volunteer</option>`,
		`<option value="Donor">Donor</option>`,
		`<option value="Beneficiary">Beneficiary</option>`,
		`<option value="Student">Student</option>`,
	} {
		if !strings.Contains(selector, want) {
			t.Errorf("Admin Add-User selector must contain %q", want)
		}
	}

	if strings.Contains(selector, `<option value="Super Admin">`) {
		t.Error("Admin Add-User selector must NOT offer Super Admin")
	}
	// The display label "Manager" must be used; the internal identifier is
	// never shown as a label.
	if strings.Contains(selector, ">Partner</option>") {
		t.Error(`role selector must render the localized label "Manager", not the internal "Partner" identifier`)
	}
}

// TestUsersPage_SuperAdminRoleSelectorShowsAllNormalRoles proves even the
// Super Admin is not offered Super Admin (no Super Admin can mint another).
func TestUsersPage_SuperAdminRoleSelectorShowsAllNormalRoles(t *testing.T) {
	selector := addUserRoleSelector(t, renderUsersContent(t, usersPageData(models.RoleSuperAdmin)))

	for _, want := range []string{
		`<option value="Admin">Admin</option>`,
		`<option value="Partner">Manager</option>`,
		`<option value="Staff">Staff</option>`,
		`<option value="Volunteer">Volunteer</option>`,
		`<option value="Donor">Donor</option>`,
		`<option value="Beneficiary">Beneficiary</option>`,
		`<option value="Student">Student</option>`,
	} {
		if !strings.Contains(selector, want) {
			t.Errorf("Super Admin Add-User selector must contain %q", want)
		}
	}

	if strings.Contains(selector, `<option value="Super Admin">`) {
		t.Error("Super Admin Add-User selector must NOT offer Super Admin")
	}
}

// TestUsersPage_ManagerRoleSelectorShowsOperationalRolesOnly proves the
// Manager (Partner role) can offer only Staff / Volunteer / Donor /
// Beneficiary / Student — never Admin, Manager or Super Admin.
func TestUsersPage_ManagerRoleSelectorShowsOperationalRolesOnly(t *testing.T) {
	selector := addUserRoleSelector(t, renderUsersContent(t, usersPageData(models.RolePartner)))

	for _, want := range []string{
		`<option value="Staff">Staff</option>`,
		`<option value="Volunteer">Volunteer</option>`,
		`<option value="Donor">Donor</option>`,
		`<option value="Beneficiary">Beneficiary</option>`,
		`<option value="Student">Student</option>`,
	} {
		if !strings.Contains(selector, want) {
			t.Errorf("Manager Add-User selector must contain %q", want)
		}
	}

	for _, forbidden := range []string{
		`<option value="Admin">`,
		`<option value="Partner">`,
		`<option value="Super Admin">`,
	} {
		if strings.Contains(selector, forbidden) {
			t.Errorf("Manager Add-User selector must NOT contain %q", forbidden)
		}
	}
}

// TestUsersPage_RoleOptionsMatchBackendHierarchy proves the selector rendered
// for ANY actor offers exactly the roles the backend would accept from that
// actor (server-side truth; the backend still re-validates on POST /users).
func TestUsersPage_RoleOptionsMatchBackendHierarchy(t *testing.T) {
	for _, actor := range models.AllRoles() {
		selector := addUserRoleSelector(t, renderUsersContent(t, usersPageData(actor)))
		authorized := services.AssignableRolesFor(actor)

		for _, target := range authorized {
			if !strings.Contains(selector, `<option value="`+target+`">`) {
				t.Errorf("actor %q: backend allows %q but the selector does not offer it", actor, target)
			}
		}

		for _, target := range models.AllRoles() {
			allowed := false
			for _, a := range authorized {
				if a == target {
					allowed = true
					break
				}
			}
			if allowed {
				continue
			}
			if strings.Contains(selector, `<option value="`+target+`">`) {
				t.Errorf("actor %q: selector offers %q which the backend rejects", actor, target)
			}
		}
	}
}

// TestUsersPage_NoHardcodedRoleAvailability proves the hardcoded role list
// ("data-roles") was removed from the Add User button: role availability is
// decided by server-side rendering plus permission gating only.
func TestUsersPage_NoHardcodedRoleAvailability(t *testing.T) {
	html := renderUsersContent(t, usersPageData(models.RoleAdmin))

	if strings.Contains(html, `data-roles="Super Admin,Admin,Partner"`) {
		t.Error("Users page must not carry the hardcoded data-roles attribute on the Add User button")
	}
}
