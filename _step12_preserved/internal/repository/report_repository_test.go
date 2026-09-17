// Package repository_test hosts database-backed tests for the report
// repository. It intentionally lives in the external test package so it can
// import internal/database (which transitively depends on internal/repository)
// without creating an import cycle.
package repository_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

// reportTestDB connects to the configured PostgreSQL database and brings it to
// the canonical state (migrations + role seed). When no database is reachable
// the tests are skipped so `go test ./...` still succeeds in plain
// environments; CI provides a database service.
func reportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("report repository test skipped (configuration unavailable): %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("report repository test skipped (database unavailable): %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("report repository test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("report repository test role seed failed: %v", err)
	}

	return db
}

// seedReportUser inserts a user with the requested status whose username and
// email carry a unique marker so report filters can isolate exactly these
// rows. Cleanup removes the rows (and any audit trail) so repeated runs never
// collide on the unique username/email indexes.
func seedReportUser(t *testing.T, db *gorm.DB, role models.Role, status, marker string) models.User {
	t.Helper()

	user := models.User{
		Username: marker + "_" + strings.ToLower(status),
		Email:    marker + "." + strings.ToLower(status) + "@pwams.local",
		FullName: "Report Test " + status,
		RoleID:   role.ID,
		Status:   status,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed report user (%s): %v", status, err)
	}

	t.Cleanup(func() {
		_ = db.Where("user_id = ?", user.ID).Delete(&models.AuditLog{}).Error
		_ = db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{}).Error
	})

	return user
}

// TestGetUserReport_PaginationClamping pins the report pagination rules
// through the exported API: the page floors at 1, the page size defaults to 20
// and clamps at 100. Clamping is observable through the returned pagination
// block.
func TestGetUserReport_PaginationClamping(t *testing.T) {
	db := reportTestDB(t)
	repo := repository.NewReportRepository(db)

	var role models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&role).Error; err != nil {
		t.Fatalf("cannot resolve Staff role: %v", err)
	}

	marker := "repoclamp_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	seedReportUser(t, db, role, models.UserStatusActive, marker)

	result, err := repo.GetUserReport(models.UserReportQuery{Search: marker, Page: -3, PageSize: 500})
	if err != nil {
		t.Fatalf("GetUserReport(clamped) failed: %v", err)
	}
	if result.Pagination.Page != 1 || result.Pagination.PageSize != 100 {
		t.Fatalf("negative page / oversized page size not clamped: %+v", result.Pagination)
	}

	result, err = repo.GetUserReport(models.UserReportQuery{Search: marker, Page: 0, PageSize: 0})
	if err != nil {
		t.Fatalf("GetUserReport(defaults) failed: %v", err)
	}
	if result.Pagination.Page != 1 || result.Pagination.PageSize != 20 {
		t.Fatalf("zero page / page size should default to 1 / 20: %+v", result.Pagination)
	}
}

// TestGetUserReport_SearchStatusAndPagination exercises the detailed user
// report end to end: the unique marker isolates the seeded rows, the KPI
// summary must reflect the seeded status distribution (Active / Disabled /
// Locked — Pending is no longer a supported user status), the role filter and
// date window must apply, and the pagination block must describe the result.
func TestGetUserReport_SearchStatusAndPagination(t *testing.T) {
	db := reportTestDB(t)
	repo := repository.NewReportRepository(db)

	var role models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&role).Error; err != nil {
		t.Fatalf("cannot resolve Staff role: %v", err)
	}

	marker := "repotest_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	seedReportUser(t, db, role, models.UserStatusActive, marker)
	seedReportUser(t, db, role, models.UserStatusDisabled, marker)
	seedReportUser(t, db, role, models.UserStatusLocked, marker)

	// Unfiltered (by marker): all three seeded rows, one per supported status.
	result, err := repo.GetUserReport(models.UserReportQuery{Search: marker})
	if err != nil {
		t.Fatalf("GetUserReport(search) failed: %v", err)
	}
	if result.Total != 3 || result.Active != 1 || result.Disabled != 1 || result.Locked != 1 {
		t.Fatalf("unexpected KPI summary: total=%d active=%d disabled=%d locked=%d, want 3/1/1/1",
			result.Total, result.Active, result.Disabled, result.Locked)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 detail rows, got %d", len(result.Rows))
	}
	if result.Pagination.TotalItems != 3 || result.Pagination.TotalPages != 1 ||
		result.Pagination.Page != 1 || result.Pagination.PageSize != 20 {
		t.Fatalf("unexpected pagination: %+v", result.Pagination)
	}
	for _, row := range result.Rows {
		if row.Role != models.RoleStaff {
			t.Fatalf("detail row role = %q, want %q", row.Role, models.RoleStaff)
		}
		if row.Status == "Pending" {
			t.Fatal("report returned a Pending user status, which is no longer supported")
		}
	}

	// Status filter narrows both the KPI summary and the detail rows.
	result, err = repo.GetUserReport(models.UserReportQuery{Search: marker, Status: models.UserStatusActive})
	if err != nil {
		t.Fatalf("GetUserReport(status filter) failed: %v", err)
	}
	if result.Total != 1 || result.Active != 1 || result.Disabled != 0 || result.Locked != 0 {
		t.Fatalf("unexpected filtered KPI summary: %+v", result)
	}
	if len(result.Rows) != 1 || result.Rows[0].Status != models.UserStatusActive || !result.Rows[0].Active {
		t.Fatalf("unexpected filtered rows: %+v", result.Rows)
	}

	// Role filter keeps all seeded Staff rows.
	result, err = repo.GetUserReport(models.UserReportQuery{Search: marker, Role: models.RoleStaff})
	if err != nil {
		t.Fatalf("GetUserReport(role filter) failed: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("role filter total = %d, want 3", result.Total)
	}

	// A date window starting tomorrow excludes every seeded row.
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	result, err = repo.GetUserReport(models.UserReportQuery{Search: marker, DateFrom: tomorrow})
	if err != nil {
		t.Fatalf("GetUserReport(date filter) failed: %v", err)
	}
	if result.Total != 0 || len(result.Rows) != 0 {
		t.Fatalf("future date window should match nothing, got total=%d rows=%d", result.Total, len(result.Rows))
	}

	// A page beyond the result set is empty but reports the real total.
	result, err = repo.GetUserReport(models.UserReportQuery{Search: marker, Page: 2})
	if err != nil {
		t.Fatalf("GetUserReport(page 2) failed: %v", err)
	}
	if len(result.Rows) != 0 || result.Pagination.Page != 2 || result.Pagination.TotalItems != 3 {
		t.Fatalf("unexpected page-2 result: rows=%d pagination=%+v", len(result.Rows), result.Pagination)
	}
}

// TestGetPersonReport_SearchStatusAndPagination exercises the person report:
// seeded persons are isolated by a unique NIC marker, KPIs must split
// Active/Inactive and the search/status filters must reach the detail rows.
func TestGetPersonReport_SearchStatusAndPagination(t *testing.T) {
	db := reportTestDB(t)
	repo := repository.NewReportRepository(db)

	var role models.Role
	if err := db.Where("name = ?", models.RoleStaff).First(&role).Error; err != nil {
		t.Fatalf("cannot resolve Staff role: %v", err)
	}

	// Persons require a creator (created_by_id NOT NULL).
	creator := seedReportUser(t, db, role, models.UserStatusActive, "repocreator_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:12])

	marker := strings.ReplaceAll(uuid.NewString(), "-", "")[:16]

	persons := []models.Person{
		{FullName: "Report Person A " + marker, NICPassport: "NIC" + marker + "A", Status: models.PersonStatusActive, CreatedByID: creator.ID},
		{FullName: "Report Person B " + marker, NICPassport: "NIC" + marker + "B", Status: models.PersonStatusInactive, CreatedByID: creator.ID},
	}
	for i := range persons {
		if err := db.Create(&persons[i]).Error; err != nil {
			t.Fatalf("failed to seed person: %v", err)
		}
	}
	t.Cleanup(func() {
		_ = db.Unscoped().Where("nic_passport IN ?", []string{persons[0].NICPassport, persons[1].NICPassport}).Delete(&models.Person{}).Error
	})

	result, err := repo.GetPersonReport(models.PersonReportQuery{Search: marker})
	if err != nil {
		t.Fatalf("GetPersonReport(search) failed: %v", err)
	}
	if result.Total != 2 || result.Active != 1 || result.Inactive != 1 {
		t.Fatalf("unexpected person KPI summary: total=%d active=%d inactive=%d, want 2/1/1",
			result.Total, result.Active, result.Inactive)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 detail rows, got %d", len(result.Rows))
	}
	if result.Pagination.TotalItems != 2 || result.Pagination.TotalPages != 1 {
		t.Fatalf("unexpected person pagination: %+v", result.Pagination)
	}

	result, err = repo.GetPersonReport(models.PersonReportQuery{Search: marker, Status: models.PersonStatusInactive})
	if err != nil {
		t.Fatalf("GetPersonReport(status filter) failed: %v", err)
	}
	if result.Total != 1 || result.Inactive != 1 || result.Active != 0 {
		t.Fatalf("unexpected filtered person summary: %+v", result)
	}
	if len(result.Rows) != 1 || result.Rows[0].Status != models.PersonStatusInactive {
		t.Fatalf("unexpected filtered person rows: %+v", result.Rows)
	}
}

// TestGetDashboardReport_ReturnsCounts verifies the dashboard report executes
// against PostgreSQL and returns populated counters (the KPI cards source).
func TestGetDashboardReport_ReturnsCounts(t *testing.T) {
	db := reportTestDB(t)
	repo := repository.NewReportRepository(db)

	report, err := repo.GetDashboardReport()
	if err != nil {
		t.Fatalf("GetDashboardReport failed: %v", err)
	}
	if report == nil {
		t.Fatal("GetDashboardReport returned nil report")
	}
	if report.TotalUsers < 0 || report.TotalPersons < 0 || report.TotalDonations < 0 {
		t.Fatalf("dashboard counters must never be negative: %+v", report)
	}
}

// TestGetDonationReport_ReturnsTotals verifies the donation report aggregate
// query executes and returns a non-nil summary.
func TestGetDonationReport_ReturnsTotals(t *testing.T) {
	db := reportTestDB(t)
	repo := repository.NewReportRepository(db)

	report, err := repo.GetDonationReport()
	if err != nil {
		t.Fatalf("GetDonationReport failed: %v", err)
	}
	if report == nil {
		t.Fatal("GetDonationReport returned nil report")
	}
}
