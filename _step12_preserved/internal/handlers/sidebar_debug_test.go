package handlers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
)

func TestSidebar_DebugAdminOutput(t *testing.T) {
	user := &models.User{Username: "tester"}
	user.Role = models.Role{Name: models.RoleAdmin}
	set := map[string]bool{
		"admin.view":                 true,
		"permission_management.view": true,
		"audit_logs.view":            true,
		"person.view":                true,
		"care.view":                  true,
		"student.view":               true,
		"donor.view":                 true,
		"donation.view":              true,
		"aid.view":                   true,
		"loan.view":                  true,
		"users.view":                 true,
		"reports.view":               true,
		"file.view":                  true,
	}
	data := localeData()
	data["current_user"] = user
	data["user_permissions"] = set
	html := renderSidebar(t, data)

	// Debug: print the full HTML
	fmt.Println("=== ADMIN SIDEBAR HTML ===")
	fmt.Println(html)
	fmt.Println("=== END ===")

	// Check for specific patterns
	if strings.Contains(html, "Administration") {
		fmt.Println("FOUND: Administration")
	} else {
		fmt.Println("NOT FOUND: Administration")
	}
	if strings.Contains(html, "/system-settings/page") {
		fmt.Println("FOUND: /system-settings/page")
	} else {
		fmt.Println("NOT FOUND: /system-settings/page")
	}
}
