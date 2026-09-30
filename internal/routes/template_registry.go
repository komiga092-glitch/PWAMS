package routes

import (
	"html/template"
	"path"

	"github.com/komiga092-glitch/pwams/internal/i18n"
)

// templateFiles is the single authoritative list of html/template files the
// PWAMS web UI is built from, relative to the repository root.
//
// Why this exists: html/template resolves `{{ template "name" }}` references
// while escaping the whole set, so ANY template that base.html's dispatch chain
// names must be loaded even when a request never renders it. When a caller
// loaded only a subset (the previous report test harness), every page came back
// as HTTP 200 with a zero-byte body and a silent
// `no such template "home_content"` error. One shared list removes that class
// of drift: the server, cmd/tplcheck, and the tests all load exactly this set.
//
// Adding a template: append it here (and nowhere else).
var templateFiles = []string{
	// Layouts + shell
	"web/templates/layouts/base.html",
	"web/templates/layouts/header.html",

	// Auth / public
	"web/templates/home.html",
	"web/templates/login.html",
	"web/templates/forgot_password.html",
	"web/templates/verify_reset_otp.html",
	"web/templates/reset_password.html",
	"web/templates/error.html",

	// Core
	"web/templates/dashboard.html",
	"web/templates/users.html",
	"web/templates/profile.html",

	// People
	"web/templates/persons.html",
	"web/templates/person_form.html",
	"web/templates/person_view.html",
	"web/templates/person_edit.html",

	// Students
	"web/templates/students.html",
	"web/templates/student_view.html",
	"web/templates/student_edit.html",

	// Donors / donations
	"web/templates/donors.html",
	"web/templates/donor_view.html",
	"web/templates/donor_edit.html",
	"web/templates/donations.html",

	// Welfare
	"web/templates/aid_requests.html",
	"web/templates/care_provided.html",

	// Finance
	"web/templates/loans.html",
	"web/templates/loan_repayments.html",
	"web/templates/revenue.html",

	// Account surfaces
	"web/templates/notifications.html",
	"web/templates/messages.html",
	"web/templates/files.html",

	// Reports
	"web/templates/reports.html",
	"web/templates/report_detail.html",

	// Administration
	"web/templates/audit_logs.html",
}

// TemplateFiles returns the authoritative template list, relative to the
// repository root. The returned slice is a copy; callers may not mutate the
// registry.
func TemplateFiles() []string {
	out := make([]string, len(templateFiles))
	copy(out, templateFiles)
	return out
}

// TemplateFilesUnder returns the same list prefixed with dir, which is how a
// caller that does not run from the repository root (a package test binary)
// reaches the same files. Pass "" for the repository root.
func TemplateFilesUnder(dir string) []string {
	files := TemplateFiles()
	if dir == "" {
		return files
	}
	for i, file := range files {
		files[i] = path.Join(dir, file)
	}
	return files
}

// TemplateFuncMap returns the helper functions every PWAMS template may use.
// It must match the functions the server installs via gin's SetFuncMap.
func TemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"t":   i18n.T,
	}
}
