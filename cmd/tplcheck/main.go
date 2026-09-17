package main

import (
	"html/template"
	"os"

	"github.com/komiga092-glitch/pwams/internal/i18n"
)

func main() {
	files := []string{
		"web/templates/layouts/base.html",
		"web/templates/layouts/header.html",
		"web/templates/home.html",
		"web/templates/login.html",
		"web/templates/forgot_password.html",
		"web/templates/verify_reset_otp.html",
		"web/templates/reset_password.html",
		"web/templates/error.html",
		"web/templates/dashboard.html",
		"web/templates/users.html",
		"web/templates/profile.html",
		"web/templates/persons.html",
		"web/templates/person_form.html",
		"web/templates/person_view.html",
		"web/templates/person_edit.html",
		"web/templates/students.html",
		"web/templates/student_view.html",
		"web/templates/student_edit.html",
		"web/templates/donors.html",
		"web/templates/donor_view.html",
		"web/templates/donor_edit.html",
		"web/templates/donations.html",
		"web/templates/aid_requests.html",
		"web/templates/care_provided.html",
		"web/templates/loans.html",
		"web/templates/loan_repayments.html",
		"web/templates/revenue.html",
		"web/templates/notifications.html",
		"web/templates/messages.html",
		"web/templates/files.html",
		"web/templates/reports.html",
		"web/templates/report_detail.html",
		"web/templates/report_page_content.html",
		"web/templates/audit_logs.html",
	}
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"t":   i18n.T,
	}
	ok := true
	for _, f := range files {
		t := template.New("check").Funcs(funcs)
		if _, err := t.ParseFiles(f); err != nil {
			println("PARSE ERROR:", f, "=>", err.Error())
			ok = false
		}
	}
	if !ok {
		os.Exit(1)
	}
	println("ALL TEMPLATES PARSED OK")
}
