package repository

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// ReportRepository owns every aggregate (summary) and row-level (paginated
// detail) query used by the Reports module. All filtering goes through
// parameterised GORM conditions; no report query ever interpolates user input
// into SQL, and no report query selects client IP addresses.
type ReportRepository struct {
	db *gorm.DB

	// columnCache memoises optional-column probes (see hasColumn) so a report
	// with several optional metrics does not re-query information_schema.
	columnCache map[string]bool

	// summaryScopes holds one rendered filter per report source table when this
	// view was produced by WithFilter. It is never mutated after construction,
	// so a filtered view may serve many requests concurrently.
	summaryScopes map[string]reportScope
}

func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{
		db:            db,
		columnCache:   map[string]bool{},
		summaryScopes: map[string]reportScope{},
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ────────────────────────────────────────────────────────────────────────────

type reportGroupCount struct {
	GroupKey string `gorm:"column:group_key"`
	Total    int64  `gorm:"column:total"`
}

type reportGroupSum struct {
	GroupKey string  `gorm:"column:group_key"`
	Total    float64 `gorm:"column:total"`
}

// groupCount returns COUNT(*) per value of column for the given table, using
// the same Table/Select/Group/Scan shape as UserRepository.CountByRole.
func (r *ReportRepository) groupCount(
	table string,
	column string,
	where string,
	args ...any,
) (map[string]int64, error) {
	var results []reportGroupCount

	where, args = r.scopedFilter(table, where, args...)

	query := r.db.
		Table(table).
		Select(fmt.Sprintf("%s AS group_key, COUNT(*) AS total", column))

	if where != "" {
		query = query.Where(where, args...)
	}

	if err := query.
		Group(column).
		Order("total DESC").
		Scan(&results).
		Error; err != nil {
		return nil, fmt.Errorf("failed to group %s by %s: %w", table, column, err)
	}

	counts := make(map[string]int64, len(results))

	for _, result := range results {
		key := strings.TrimSpace(result.GroupKey)
		if key == "" {
			key = "Unspecified"
		}
		counts[key] = result.Total
	}

	return counts, nil
}

// groupSum returns SUM(column) per value of groupColumn.
func (r *ReportRepository) groupSum(
	table string,
	column string,
	groupColumn string,
	where string,
	args ...any,
) (map[string]float64, error) {
	var results []reportGroupSum

	where, args = r.scopedFilter(table, where, args...)

	query := r.db.
		Table(table).
		Select(fmt.Sprintf("%s AS group_key, COALESCE(SUM(%s), 0) AS total", groupColumn, column))

	if where != "" {
		query = query.Where(where, args...)
	}

	if err := query.
		Group(groupColumn).
		Order("total DESC").
		Scan(&results).
		Error; err != nil {
		return nil, fmt.Errorf("failed to sum %s.%s by %s: %w", table, column, groupColumn, err)
	}

	totals := make(map[string]float64, len(results))

	for _, result := range results {
		key := strings.TrimSpace(result.GroupKey)
		if key == "" {
			key = "Unspecified"
		}
		totals[key] = result.Total
	}

	return totals, nil
}

// groupUserCountByRole returns COUNT(*) per role *name* by joining roles, so
// the Users/Account Status reports never expose raw role UUIDs to the UI.
func (r *ReportRepository) groupUserCountByRole() (map[string]int64, error) {
	var results []reportGroupCount

	// The summary scope is rendered with the users table prefix because roles
	// carries its own status/created_at columns.
	where := "users.deleted_at IS NULL"
	var args []any

	if scope, ok := r.summaryScopes["users"]; ok {
		if clause, scopeArgs := scope.render("users."); clause != "" {
			where = where + " AND (" + clause + ")"
			args = append(args, scopeArgs...)
		}
	}

	err := r.db.
		Table("users").
		Select("roles.name AS group_key, COUNT(users.id) AS total").
		Joins("JOIN roles ON roles.id = users.role_id").
		Where(where, args...).
		Group("roles.name").
		Order("total DESC").
		Scan(&results).
		Error

	if err != nil {
		return nil, fmt.Errorf("failed to group users by role name: %w", err)
	}

	counts := make(map[string]int64, len(results))

	for _, result := range results {
		key := strings.TrimSpace(result.GroupKey)
		if key == "" {
			key = "Unspecified"
		}
		counts[key] = result.Total
	}

	return counts, nil
}

// reportTableScope returns the soft-delete scope applied to every aggregate
// on a report source table, so summary numbers always match the row-level
// detail tables (which use typed models and therefore GORM soft deletes).
// users carries GORM DeletedAt; persons/students/donors/donations/aid
// requests/revenue carry both DeletedAt and the is_deleted flag; loans,
// loan repayments and care provided carry only the is_deleted flag;
// audit_logs has neither (and must never gain IP-based scoping).
func reportTableScope(table string) string {
	switch table {
	case "users":
		return "(deleted_at IS NULL)"
	case "persons", "students", "donors", "donations", "aid_requests", "revenue_records":
		return "(deleted_at IS NULL AND is_deleted = FALSE)"
	case "loans", "loan_repayments", "care_provided":
		return "(is_deleted = FALSE)"
	}
	return ""
}

// scopedWhere combines an explicit parameterised filter with the table's
// soft-delete scope.
func scopedWhere(table, where string) string {
	scope := reportTableScope(table)

	switch {
	case scope == "":
		return where
	case where == "":
		return scope
	default:
		return scope + " AND (" + where + ")"
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Summary filter scoping
//
// A report's summary cards and its detail table must describe the same rows:
// the report reflects only data inside the specified range/filter, so cards
// that ignore the filter while the table honours it are a defect. The types
// below render the shared models.ReportFilter as a parameterised WHERE fragment
// per report source table, and WithFilter hands the aggregate helpers a
// repository view that applies it. User input always travels as a bound
// argument — no fragment is ever interpolated from request data.
// ────────────────────────────────────────────────────────────────────────────

// reportSummarySource maps one report source table to the columns the shared
// filter narrows. Every entry mirrors the filter columns of that table's detail
// query (Get*ReportRows), so a summary can never count rows the detail table
// hides.
type reportSummarySource struct {
	dateColumn     string
	statusColumn   string
	typeColumn     string
	priorityColumn string
	gradeColumn    string
	roleScoped     bool
	searchColumns  []string
}

// reportSummarySources lists every filterable report source table. Each column
// here is also used by the matching Get*ReportRows query.
var reportSummarySources = map[string]reportSummarySource{
	"users": {
		dateColumn:    "created_at",
		statusColumn:  "status",
		roleScoped:    true,
		searchColumns: []string{"username", "email"},
	},
	"persons": {
		dateColumn:    "created_at",
		statusColumn:  "status",
		searchColumns: []string{"full_name", "nic_passport"},
	},
	"students": {
		dateColumn:    "created_at",
		statusColumn:  "status",
		gradeColumn:   "grade",
		searchColumns: []string{"full_name", "student_code", "school_name"},
	},
	"donors": {
		dateColumn:    "created_at",
		statusColumn:  "status",
		typeColumn:    "donor_type",
		searchColumns: []string{"name", "email"},
	},
	"donations": {
		dateColumn:    "donation_date",
		statusColumn:  "status",
		typeColumn:    "donation_type",
		searchColumns: []string{"item_name", "reference_no", "description"},
	},
	"aid_requests": {
		dateColumn:     "request_date",
		statusColumn:   "status",
		typeColumn:     "aid_type",
		priorityColumn: "priority",
		searchColumns:  []string{"title", "description"},
	},
	"care_provided": {
		dateColumn:    "provided_at",
		statusColumn:  "status",
		typeColumn:    "care_type",
		searchColumns: []string{"description", "care_type", "provided_by"},
	},
	"loans": {
		dateColumn:    "created_at",
		statusColumn:  "status",
		searchColumns: []string{"purpose"},
	},
	"loan_repayments": {
		dateColumn:    "due_date",
		statusColumn:  "status",
		searchColumns: []string{"payment_reference", "notes"},
	},
	"revenue_records": {
		dateColumn:    "record_date",
		statusColumn:  "category",
		typeColumn:    "record_type",
		searchColumns: []string{"description", "category", "reference_no"},
	},
	"audit_logs": {
		dateColumn:    "created_at",
		statusColumn:  "entity",
		typeColumn:    "action",
		searchColumns: []string{"action", "entity", "details"},
	},
}

type reportScopeEquals struct {
	column string
	value  string
}

type reportScopeDate struct {
	column  string
	from    time.Time
	to      time.Time
	hasFrom bool
	hasTo   bool
}

// reportScope is one report source table's filter in renderable form.
type reportScope struct {
	roleName string
	date     *reportScopeDate
	equals   []reportScopeEquals
	search   []string
	pattern  string
}

// empty reports whether the scope narrows nothing.
func (s reportScope) empty() bool {
	return s.roleName == "" && s.date == nil && len(s.equals) == 0 && len(s.search) == 0
}

// render turns the scope into a parameterised WHERE fragment. prefix qualifies
// every column (for example "users.") for aggregates that join another table so
// an unqualified column can never become ambiguous.
func (s reportScope) render(prefix string) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 8)

	if s.roleName != "" {
		clauses = append(
			clauses,
			prefix+"role_id IN (SELECT id FROM roles WHERE LOWER(name) = LOWER(?))",
		)
		args = append(args, s.roleName)
	}

	if s.date != nil {
		if s.date.hasFrom {
			clauses = append(clauses, prefix+s.date.column+" >= ?")
			args = append(args, s.date.from)
		}
		if s.date.hasTo {
			clauses = append(clauses, prefix+s.date.column+" <= ?")
			args = append(args, s.date.to)
		}
	}

	for _, equals := range s.equals {
		clauses = append(clauses, "LOWER("+prefix+equals.column+") = LOWER(?)")
		args = append(args, equals.value)
	}

	if len(s.search) > 0 && s.pattern != "" {
		matches := make([]string, 0, len(s.search))
		for _, column := range s.search {
			matches = append(matches, "LOWER("+prefix+column+") LIKE ?")
			args = append(args, s.pattern)
		}
		clauses = append(clauses, "("+strings.Join(matches, " OR ")+")")
	}

	return strings.Join(clauses, " AND "), args
}

// buildReportScope renders filter into the scope of one report source table.
// Tables the filter cannot narrow yield an empty scope.
func buildReportScope(table string, filter models.ReportFilter) (reportScope, error) {
	source, ok := reportSummarySources[table]
	if !ok {
		return reportScope{}, nil
	}

	scope := reportScope{}

	if source.dateColumn != "" && (filter.From != "" || filter.To != "") {
		dates := &reportScopeDate{column: source.dateColumn}

		if filter.From != "" {
			parsed, err := time.Parse("2006-01-02", filter.From)
			if err != nil {
				return reportScope{}, fmt.Errorf("failed to parse from date: %w", err)
			}
			dates.from, dates.hasFrom = parsed, true
		}

		if filter.To != "" {
			parsed, err := time.Parse("2006-01-02", filter.To)
			if err != nil {
				return reportScope{}, fmt.Errorf("failed to parse to date: %w", err)
			}
			// Same inclusive whole-day range applyDateRange gives detail rows.
			dates.to, dates.hasTo = parsed.Add(24*time.Hour).Add(-time.Nanosecond), true
		}

		scope.date = dates
	}

	addEquals := func(column, value string) {
		value = strings.TrimSpace(value)
		if column == "" || value == "" {
			return
		}
		scope.equals = append(scope.equals, reportScopeEquals{column: column, value: value})
	}

	addEquals(source.statusColumn, filter.Status)
	addEquals(source.typeColumn, filter.Type)
	addEquals(source.priorityColumn, filter.Priority)
	addEquals(source.gradeColumn, filter.Grade)

	if source.roleScoped {
		scope.roleName = strings.TrimSpace(filter.Role)
	}

	if strings.TrimSpace(filter.Query) != "" && len(source.searchColumns) > 0 {
		scope.search = source.searchColumns
		scope.pattern = reportContains(filter.Query)
	}

	return scope, nil
}

// WithFilter returns a repository view whose aggregate queries honour filter so
// a report's summary cards describe exactly the rows its detail table shows.
// The receiver is never mutated — the shared repository instance keeps serving
// unfiltered summaries concurrently — and an empty filter returns the receiver
// unchanged, which keeps unfiltered reports on their original query plan.
func (r *ReportRepository) WithFilter(filter models.ReportFilter) (*ReportRepository, error) {
	scopes := make(map[string]reportScope, len(reportSummarySources))

	for table := range reportSummarySources {
		scope, err := buildReportScope(table, filter)
		if err != nil {
			return nil, err
		}
		if !scope.empty() {
			scopes[table] = scope
		}
	}

	if len(scopes) == 0 {
		return r, nil
	}

	return &ReportRepository{
		db:            r.db,
		columnCache:   r.columnCache,
		summaryScopes: scopes,
	}, nil
}

// scopedFilter merges the summary scope bound to this repository view (when the
// view was built by WithFilter) with an explicit parameterised filter and the
// table's soft-delete scope. The returned where/args pair is always safe to use
// together, even when either side is empty.
func (r *ReportRepository) scopedFilter(table, where string, args ...any) (string, []any) {
	scope, ok := r.summaryScopes[table]
	if !ok {
		return scopedWhere(table, where), args
	}

	clause, scopeArgs := scope.render("")
	if clause == "" {
		return scopedWhere(table, where), args
	}

	mergedArgs := make([]any, 0, len(args)+len(scopeArgs))
	mergedArgs = append(mergedArgs, args...)
	mergedArgs = append(mergedArgs, scopeArgs...)

	if where == "" {
		where = clause
	} else {
		where = where + " AND (" + clause + ")"
	}

	return scopedWhere(table, where), mergedArgs
}

// countWhere returns COUNT(*) for table with an optional parameterised filter.
func (r *ReportRepository) countWhere(
	table string,
	where string,
	args ...any,
) (int64, error) {
	var total int64

	where, args = r.scopedFilter(table, where, args...)

	query := r.db.Table(table)

	if where != "" {
		query = query.Where(where, args...)
	}

	if err := query.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("failed to count %s: %w", table, err)
	}

	return total, nil
}

// sumWhere returns COALESCE(SUM(column), 0) for table with a parameterised filter.
func (r *ReportRepository) sumWhere(
	table string,
	column string,
	where string,
	args ...any,
) (float64, error) {
	var total float64

	where, args = r.scopedFilter(table, where, args...)

	query := r.db.Table(table).Select(fmt.Sprintf("COALESCE(SUM(%s), 0)", column))

	if where != "" {
		query = query.Where(where, args...)
	}

	if err := query.Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("failed to sum %s.%s: %w", table, column, err)
	}

	return total, nil
}

// hasColumn reports whether table has column, memoised per repository. Used
// only for metrics that depend on optional/nullable columns so a report never
// fails when a deployment carries a slightly older schema.
func (r *ReportRepository) hasColumn(table, column string) bool {
	key := table + "." + column

	if present, ok := r.columnCache[key]; ok {
		return present
	}

	var matches int64

	err := r.db.
		Raw(
			`SELECT COUNT(*) FROM information_schema.columns
			 WHERE table_schema = current_schema()
			   AND table_name = ?
			   AND column_name = ?`,
			table,
			column,
		).
		Scan(&matches).
		Error

	present := err == nil && matches > 0
	r.columnCache[key] = present

	return present
}

// applyDateRange narrows query to column BETWEEN from (00:00) and to (23:59:59).
// Dates are parsed strictly; input is validated by the service layer before it
// reaches the repository and re-validated here.
func applyDateRange(
	query *gorm.DB,
	column string,
	from string,
	to string,
) (*gorm.DB, error) {
	if from != "" {
		parsed, err := time.Parse("2006-01-02", from)
		if err != nil {
			return nil, fmt.Errorf("failed to parse from date: %w", err)
		}

		query = query.Where(column+" >= ?", parsed)
	}

	if to != "" {
		parsed, err := time.Parse("2006-01-02", to)
		if err != nil {
			return nil, fmt.Errorf("failed to parse to date: %w", err)
		}

		endOfDay := parsed.Add(24 * time.Hour).Add(-time.Nanosecond)
		query = query.Where(column+" <= ?", endOfDay)
	}

	return query, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Aggregate summaries
// ────────────────────────────────────────────────────────────────────────────

func (r *ReportRepository) GetDashboardReport() (*models.DashboardReport, error) {
	report := &models.DashboardReport{}

	counts := []struct {
		table string
		where string
		dest  *int64
	}{
		{table: "users", dest: &report.TotalUsers},
		{table: "persons", dest: &report.TotalPersons},
		{table: "students", dest: &report.TotalStudents},
		{table: "donors", dest: &report.TotalDonors},
		{table: "donations", dest: &report.TotalDonations},
		{table: "aid_requests", dest: &report.TotalAidRequests},
		{table: "care_provided", dest: &report.TotalCareProvided},
	}

	for _, item := range counts {
		total, err := r.countWhere(item.table, item.where)
		if err != nil {
			return nil, err
		}
		*item.dest = total
	}

	return report, nil
}

func (r *ReportRepository) GetUsersReport() (*models.UsersReport, error) {
	report := &models.UsersReport{}

	byStatus, err := r.groupCount("users", "status", "")
	if err != nil {
		return nil, err
	}

	byRole, err := r.groupUserCountByRole()
	if err != nil {
		return nil, err
	}

	report.ByStatus = byStatus
	report.ByRole = byRole

	for status, total := range byStatus {
		report.TotalUsers += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.UserStatusActive):
			report.ActiveUsers += total
		case strings.ToLower(models.UserStatusDisabled):
			report.DisabledUsers += total
		case strings.ToLower(models.UserStatusLocked):
			report.LockedUsers += total
		}
	}

	if report.TotalUsers == 0 {
		total, err := r.countWhere("users", "")
		if err != nil {
			return nil, err
		}
		report.TotalUsers = total
	}

	newUsers, err := r.countWhere(
		"users",
		"created_at >= ?",
		time.Now().AddDate(0, 0, -30),
	)
	if err != nil {
		return nil, err
	}
	report.NewLast30Days = newUsers

	return report, nil
}

func (r *ReportRepository) GetAccountStatusReport() (*models.AccountStatusReport, error) {
	report := &models.AccountStatusReport{}

	users, err := r.GetUsersReport()
	if err != nil {
		return nil, err
	}

	report.TotalUsers = users.TotalUsers
	report.ActiveUsers = users.ActiveUsers
	report.DisabledUsers = users.DisabledUsers
	report.LockedUsers = users.LockedUsers
	report.ByStatus = users.ByStatus
	report.ByRole = users.ByRole

	if r.hasColumn("users", "locked_until") {
		lockedNow, err := r.countWhere(
			"users",
			"locked_until IS NOT NULL AND locked_until > ?",
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		report.LockedNow = lockedNow
	} else {
		report.LockedNow = report.LockedUsers
	}

	if r.hasColumn("users", "last_login_at") {
		never, err := r.countWhere("users", "last_login_at IS NULL")
		if err != nil {
			return nil, err
		}
		report.NeverLoggedIn = never
	}

	return report, nil
}

func (r *ReportRepository) GetPersonReport() (*models.PersonReport, error) {
	report := &models.PersonReport{}

	byStatus, err := r.groupCount("persons", "status", "")
	if err != nil {
		return nil, err
	}

	byGender, err := r.groupCount("persons", "gender", "")
	if err != nil {
		return nil, err
	}

	report.ByStatus = byStatus
	report.ByGender = byGender

	for status, total := range byStatus {
		report.TotalPersons += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.PersonStatusActive):
			report.ActivePersons += total
		case strings.ToLower(models.PersonStatusInactive):
			report.InactivePersons += total
		case strings.ToLower(models.PersonStatusPending):
			report.PendingPersons += total
		}
	}

	if report.TotalPersons == 0 {
		total, err := r.countWhere("persons", "")
		if err != nil {
			return nil, err
		}
		report.TotalPersons = total
	}

	return report, nil
}

func (r *ReportRepository) GetStudentReport() (*models.StudentReport, error) {
	report := &models.StudentReport{}

	byStatus, err := r.groupCount("students", "status", "")
	if err != nil {
		return nil, err
	}

	byGrade, err := r.groupCount("students", "grade", "")
	if err != nil {
		return nil, err
	}

	byAcademicYear, err := r.groupCount("students", "CAST(academic_year AS TEXT)", "")
	if err != nil {
		return nil, err
	}

	if byAcademicYear == nil {
		byAcademicYear = map[string]int64{}
	}
	report.ByAcademicYear = byAcademicYear

	report.ByStatus = byStatus
	report.ByGrade = byGrade

	for status, total := range byStatus {
		report.TotalStudents += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.StudentStatusActive):
			report.ActiveStudents += total
		case strings.ToLower(models.StudentStatusInactive):
			report.InactiveStudents += total
		case strings.ToLower(models.StudentStatusPending):
			report.PendingStudents += total
		}
	}

	if report.TotalStudents == 0 {
		total, err := r.countWhere("students", "")
		if err != nil {
			return nil, err
		}
		report.TotalStudents = total
	}

	recent, err := r.countWhere(
		"students",
		"created_at >= ?",
		time.Now().AddDate(0, 0, -30),
	)
	if err != nil {
		return nil, err
	}
	report.RecentStudents = recent

	return report, nil
}
func (r *ReportRepository) GetDonorReport() (*models.DonorReport, error) {
	report := &models.DonorReport{}

	byStatus, err := r.groupCount("donors", "status", "")
	if err != nil {
		return nil, err
	}

	byType, err := r.groupCount("donors", "donor_type", "")
	if err != nil {
		return nil, err
	}

	report.ByStatus = byStatus
	report.ByType = byType

	for status, total := range byStatus {
		report.TotalDonors += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.DonorStatusActive):
			report.ActiveDonors += total
		case strings.ToLower(models.DonorStatusInactive):
			report.InactiveDonors += total
		case strings.ToLower(models.DonorStatusPending):
			report.PendingDonors += total
		}
	}

	if report.TotalDonors == 0 {
		total, err := r.countWhere("donors", "")
		if err != nil {
			return nil, err
		}
		report.TotalDonors = total
	}

	donationCount, err := r.countWhere("donations", "")
	if err != nil {
		return nil, err
	}
	report.TotalDonations = donationCount

	donationAmount, err := r.sumWhere("donations", "amount", "")
	if err != nil {
		return nil, err
	}
	report.TotalDonationAmount = donationAmount

	return report, nil
}

func (r *ReportRepository) GetPersonReportRows(
	filter models.ReportFilter,
) ([]models.PersonReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.Person{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(full_name) LIKE ? OR LOWER(nic_passport) LIKE ?",
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	query, err := applyDateRange(query, "created_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count person report rows: %w", err)
	}

	var persons []models.Person
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&persons).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list person report rows: %w", err)
	}

	rows := make([]models.PersonReportRow, 0, len(persons))
	for _, person := range persons {
		rows = append(rows, models.PersonReportRow{
			ID:          person.ID,
			FullName:    person.FullName,
			NICPassport: person.NICPassport,
			Gender:      person.Gender,
			Status:      person.Status,
			CreatedAt:   person.CreatedAt,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetStudentReportRows(
	filter models.ReportFilter,
) ([]models.StudentReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.Student{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(full_name) LIKE ? OR LOWER(student_code) LIKE ? OR LOWER(school_name) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	if strings.TrimSpace(filter.Grade) != "" {
		query = query.Where("LOWER(grade) = LOWER(?)", strings.TrimSpace(filter.Grade))
	}

	query, err := applyDateRange(query, "created_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count student report rows: %w", err)
	}

	var students []models.Student
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&students).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list student report rows: %w", err)
	}

	rows := make([]models.StudentReportRow, 0, len(students))
	for _, student := range students {
		rows = append(rows, models.StudentReportRow{
			ID:           student.ID,
			FullName:     student.FullName,
			StudentCode:  student.StudentCode,
			SchoolName:   student.SchoolName,
			Grade:        student.Grade,
			AcademicYear: student.AcademicYear,
			Status:       student.Status,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetDonationReport() (*models.DonationReport, error) {
	report := &models.DonationReport{}

	byStatus, err := r.groupCount("donations", "status", "")
	if err != nil {
		return nil, err
	}

	report.ByStatus = byStatus

	for status, total := range byStatus {
		report.TotalDonations += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.DonationStatusPending):
			report.PendingDonations += total
		case strings.ToLower(models.DonationStatusConfirmed):
			report.ConfirmedDonations += total
		case strings.ToLower(models.DonationStatusCancelled):
			report.CancelledDonations += total
		}
	}

	byType, err := r.groupCount("donations", "donation_type", "")
	if err != nil {
		return nil, err
	}
	report.ByType = byType

	if report.TotalDonations == 0 {
		total, err := r.countWhere("donations", "")
		if err != nil {
			return nil, err
		}
		report.TotalDonations = total
	}

	totalAmount, err := r.sumWhere("donations", "amount", "")
	if err != nil {
		return nil, err
	}
	report.TotalAmount = totalAmount

	return report, nil
}

func (r *ReportRepository) GetCareProvidedReport() (*models.CareProvidedReport, error) {
	report := &models.CareProvidedReport{}

	const activeFilter = "is_deleted = FALSE"

	byStatus, err := r.groupCount("care_provided", "status", activeFilter)
	if err != nil {
		return nil, err
	}

	byType, err := r.groupCount("care_provided", "care_type", activeFilter)
	if err != nil {
		return nil, err
	}

	report.ByStatus = byStatus
	report.ByType = byType

	for status, total := range byStatus {
		report.TotalRecords += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.CareProvidedStatusCompleted):
			report.CompletedRecords += total
		case strings.ToLower(models.CareProvidedStatusPending):
			report.PendingRecords += total
		case strings.ToLower(models.CareProvidedStatusCancelled):
			report.CancelledRecords += total
		}
	}

	if report.TotalRecords == 0 {
		total, err := r.countWhere("care_provided", activeFilter)
		if err != nil {
			return nil, err
		}
		report.TotalRecords = total
	}

	totalAmount, err := r.sumWhere("care_provided", "amount", activeFilter)
	if err != nil {
		return nil, err
	}
	report.TotalAmount = totalAmount

	return report, nil
}

func (r *ReportRepository) GetLoanReport() (*models.LoanReport, error) {
	report := &models.LoanReport{}

	byStatus, err := r.groupCount("loans", "status", "")
	if err != nil {
		return nil, err
	}

	report.ByStatus = byStatus

	for status, total := range byStatus {
		report.TotalLoans += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.LoanStatusActive):
			report.ActiveLoans += total
		case strings.ToLower(models.LoanStatusPending):
			report.PendingLoans += total
		case strings.ToLower(models.LoanStatusApproved):
			report.ApprovedLoans += total
		case strings.ToLower(models.LoanStatusCompleted):
			report.CompletedLoans += total
		case strings.ToLower(models.LoanStatusRejected):
			report.RejectedLoans += total
		case strings.ToLower(models.LoanStatusCancelled):
			report.CancelledLoans += total
		}
	}

	if report.TotalLoans == 0 {
		total, err := r.countWhere("loans", "")
		if err != nil {
			return nil, err
		}
		report.TotalLoans = total
	}

	disbursed, err := r.sumWhere(
		"loans",
		"loan_amount",
		"LOWER(status) IN (?, ?, ?)",
		strings.ToLower(models.LoanStatusApproved),
		strings.ToLower(models.LoanStatusActive),
		strings.ToLower(models.LoanStatusCompleted),
	)
	if err != nil {
		return nil, err
	}
	report.TotalDisbursed = disbursed

	outstanding, err := r.outstandingRepayments()
	if err != nil {
		return nil, err
	}
	report.OutstandingAmount = outstanding

	return report, nil
}

// outstandingRepayments sums the unsettled portion of every scheduled
// installment (Pending/Overdue). Shared by the Loans and Loan Repayments
// reports so the two areas cannot drift apart.
func (r *ReportRepository) outstandingRepayments() (float64, error) {
	return r.sumWhere(
		"loan_repayments",
		"amount - paid_amount",
		"LOWER(status) IN (?, ?)",
		strings.ToLower(models.RepaymentStatusPending),
		strings.ToLower(models.RepaymentStatusOverdue),
	)
}

func (r *ReportRepository) GetLoanRepaymentReport() (*models.LoanRepaymentReport, error) {
	report := &models.LoanRepaymentReport{}

	byStatus, err := r.groupCount("loan_repayments", "status", "")
	if err != nil {
		return nil, err
	}

	report.ByStatus = byStatus

	for status, total := range byStatus {
		report.TotalRepayments += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.RepaymentStatusPaid):
			report.PaidRepayments += total
		case strings.ToLower(models.RepaymentStatusPending):
			report.PendingRepayments += total
		case strings.ToLower(models.RepaymentStatusOverdue):
			report.OverdueRepayments += total
		case strings.ToLower(models.RepaymentStatusCancelled):
			report.CancelledRepayments += total
		}
	}

	if report.TotalRepayments == 0 {
		total, err := r.countWhere("loan_repayments", "")
		if err != nil {
			return nil, err
		}
		report.TotalRepayments = total
	}

	totalAmount, err := r.sumWhere("loan_repayments", "amount", "")
	if err != nil {
		return nil, err
	}
	report.TotalAmount = totalAmount

	paidAmount, err := r.sumWhere("loan_repayments", "paid_amount", "")
	if err != nil {
		return nil, err
	}
	report.TotalPaidAmount = paidAmount

	outstanding, err := r.outstandingRepayments()
	if err != nil {
		return nil, err
	}
	report.OutstandingAmount = outstanding

	return report, nil
}

func (r *ReportRepository) GetAidRequestReport() (*models.AidRequestReport, error) {
	report := &models.AidRequestReport{}

	byStatus, err := r.groupCount("aid_requests", "status", "")
	if err != nil {
		return nil, err
	}

	byType, err := r.groupCount("aid_requests", "aid_type", "")
	if err != nil {
		return nil, err
	}

	byPriority, err := r.groupCount("aid_requests", "priority", "")
	if err != nil {
		return nil, err
	}

	report.ByType = byType
	report.ByPriority = byPriority

	for status, total := range byStatus {
		report.TotalRequests += total

		switch strings.ToLower(status) {
		case strings.ToLower(models.AidStatusPending):
			report.PendingRequests += total
		case strings.ToLower(models.AidStatusUnderReview):
			report.UnderReviewCount += total
		case strings.ToLower(models.AidStatusApproved):
			report.ApprovedRequests += total
		case strings.ToLower(models.AidStatusRejected):
			report.RejectedRequests += total
		case strings.ToLower(models.AidStatusCompleted):
			report.CompletedRequests += total
		case strings.ToLower(models.AidStatusCancelled):
			report.CancelledRequests += total
		}
	}

	if report.TotalRequests == 0 {
		total, err := r.countWhere("aid_requests", "")
		if err != nil {
			return nil, err
		}
		report.TotalRequests = total
	}

	requested, err := r.sumWhere("aid_requests", "requested_amount", "")
	if err != nil {
		return nil, err
	}
	report.TotalRequested = requested

	approved, err := r.sumWhere("aid_requests", "approved_amount", "")
	if err != nil {
		return nil, err
	}
	report.TotalApproved = approved

	return report, nil
}

func (r *ReportRepository) GetRevenueReport() (*models.RevenueReport, error) {
	report := &models.RevenueReport{}

	income, err := r.sumWhere(
		"revenue_records",
		"amount",
		"record_type = ?",
		models.RevenueTypeIncome,
	)
	if err != nil {
		return nil, err
	}
	report.TotalIncome = income

	expenses, err := r.sumWhere(
		"revenue_records",
		"amount",
		"record_type = ?",
		models.RevenueTypeExpense,
	)
	if err != nil {
		return nil, err
	}
	report.TotalExpenses = expenses
	report.Net = income - expenses

	byCategory, err := r.groupSum(
		"revenue_records",
		"CASE WHEN record_type = 'income' THEN amount ELSE -amount END",
		"category",
		"",
	)
	if err != nil {
		return nil, err
	}
	report.ByCategory = byCategory

	total, err := r.countWhere("revenue_records", "")
	if err != nil {
		return nil, err
	}
	report.TotalRecords = total

	return report, nil
}

// GetAuditLogReport aggregates audit entries by action and entity. The
// audit_logs table has neither a soft-delete column nor IP addresses, so the
// queries need no soft-delete scoping — and must never gain an IP filter.
func (r *ReportRepository) GetAuditLogReport() (*models.AuditLogReport, error) {
	report := &models.AuditLogReport{}

	total, err := r.countWhere("audit_logs", "")
	if err != nil {
		return nil, err
	}
	report.TotalEntries = total

	byAction, err := r.groupCount("audit_logs", "action", "")
	if err != nil {
		return nil, err
	}
	report.ByAction = byAction

	byEntity, err := r.groupCount("audit_logs", "entity", "")
	if err != nil {
		return nil, err
	}
	report.ByEntity = byEntity

	return report, nil
}

// GetSystemAlertReport derives alert counts from audit entries. PWAMS has no
// dedicated alert table, so alerts are audit rows whose action is in
// models.SystemAlertActions. No IP address is read or returned.
func (r *ReportRepository) GetSystemAlertReport() (*models.SystemAlertReport, error) {
	report := &models.SystemAlertReport{}

	placeholders := make([]string, 0, len(models.SystemAlertActions))
	args := make([]any, 0, len(models.SystemAlertActions))

	for _, action := range models.SystemAlertActions {
		placeholders = append(placeholders, "?")
		args = append(args, action)
	}

	alertFilter := "action IN (" + strings.Join(placeholders, ", ") + ")"

	total, err := r.countWhere("audit_logs", alertFilter, args...)
	if err != nil {
		return nil, err
	}
	report.TotalAlerts = total

	byAction, err := r.groupCount("audit_logs", "action", alertFilter, args...)
	if err != nil {
		return nil, err
	}
	report.ByAction = byAction

	byEntity, err := r.groupCount("audit_logs", "entity", alertFilter, args...)
	if err != nil {
		return nil, err
	}
	report.ByEntity = byEntity

	for action, count := range byAction {
		switch strings.ToUpper(action) {
		case "LOGIN_FAILED":
			report.FailedLogins += count
		case "ACCOUNT_LOCKED":
			report.LockedAccounts += count
		case "ACCESS_DENIED", "AUTHORIZATION_DENIED":
			report.DeniedAccess += count
		}
	}

	recentFilter := alertFilter + " AND created_at >= ?"
	recentArgs := append(append([]any{}, args...), time.Now().UTC().AddDate(0, 0, -7))

	recent, err := r.countWhere("audit_logs", recentFilter, recentArgs...)
	if err != nil {
		return nil, err
	}
	report.RecentAlerts = recent

	return report, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Row-level (paginated) detail queries
//
// Every method below takes the validated models.ReportFilter and applies it as
// parameterised GORM conditions. Pagination bounds are re-clamped here so the
// repository is safe even when called directly from a test.
// ────────────────────────────────────────────────────────────────────────────

// normalizeReportPage clamps report pagination to safe bounds: page >= 1 and
// 1 <= page_size <= 100, so a report can never stream an unbounded table.
func normalizeReportPage(filter models.ReportFilter) (page, pageSize, offset int) {
	page = filter.Page
	if page < 1 {
		page = 1
	}

	pageSize = filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return page, pageSize, (page - 1) * pageSize
}

// reportContains builds a case-insensitive LIKE pattern (never interpolated
// into SQL — always passed as a bound parameter).
func reportContains(value string) string {
	return "%" + strings.ToLower(strings.TrimSpace(value)) + "%"
}

func (r *ReportRepository) GetUserReportRows(
	filter models.ReportFilter,
) ([]models.UserReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.User{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(username) LIKE ? OR LOWER(email) LIKE ?",
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	if strings.TrimSpace(filter.Role) != "" {
		query = query.Where(
			"role_id IN (SELECT id FROM roles WHERE LOWER(name) = LOWER(?))",
			strings.TrimSpace(filter.Role),
		)
	}

	query, err := applyDateRange(query, "created_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users report rows: %w", err)
	}

	var users []models.User
	if err := query.
		Preload("Role").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&users).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list users report rows: %w", err)
	}

	rows := make([]models.UserReportRow, 0, len(users))

	for _, user := range users {
		rows = append(rows, models.UserReportRow{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			RoleName:    user.Role.Name,
			Status:      user.Status,
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetDonorReportRows(
	filter models.ReportFilter,
) ([]models.DonorReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.Donor{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(email) LIKE ?",
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	if strings.TrimSpace(filter.Type) != "" {
		query = query.Where("LOWER(donor_type) = LOWER(?)", strings.TrimSpace(filter.Type))
	}

	query, err := applyDateRange(query, "created_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count donor report rows: %w", err)
	}

	var donors []models.Donor
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&donors).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list donor report rows: %w", err)
	}

	rows := make([]models.DonorReportRow, 0, len(donors))
	for _, donor := range donors {
		rows = append(rows, models.DonorReportRow{
			ID:        donor.ID,
			Name:      donor.Name,
			DonorType: donor.DonorType,
			Phone:     donor.Phone,
			Status:    donor.Status,
			CreatedAt: donor.CreatedAt,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetDonationReportRows(
	filter models.ReportFilter,
) ([]models.DonationReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.Donation{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(item_name) LIKE ? OR LOWER(reference_no) LIKE ? OR LOWER(description) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	if strings.TrimSpace(filter.Type) != "" {
		query = query.Where("LOWER(donation_type) = LOWER(?)", strings.TrimSpace(filter.Type))
	}

	query, err := applyDateRange(query, "donation_date", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count donation report rows: %w", err)
	}

	var donations []models.Donation
	if err := query.
		Preload("Donor").
		Order("donation_date DESC, created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&donations).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list donation report rows: %w", err)
	}

	rows := make([]models.DonationReportRow, 0, len(donations))
	for _, donation := range donations {
		rows = append(rows, models.DonationReportRow{
			ID:           donation.ID,
			DonorName:    donation.Donor.Name,
			DonationType: donation.DonationType,
			ItemName:     donation.ItemName,
			Amount:       donation.Amount,
			Currency:     donation.Currency,
			DonationDate: donation.DonationDate,
			Status:       donation.Status,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetAidRequestReportRows(
	filter models.ReportFilter,
) ([]models.AidRequestReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.AidRequest{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(title) LIKE ? OR LOWER(description) LIKE ?",
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	if strings.TrimSpace(filter.Priority) != "" {
		query = query.Where("LOWER(priority) = LOWER(?)", strings.TrimSpace(filter.Priority))
	}

	if strings.TrimSpace(filter.Type) != "" {
		query = query.Where("LOWER(aid_type) = LOWER(?)", strings.TrimSpace(filter.Type))
	}

	query, err := applyDateRange(query, "request_date", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count aid request report rows: %w", err)
	}

	var aidRequests []models.AidRequest
	if err := query.
		Preload("Person").
		Order("request_date DESC, created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&aidRequests).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list aid request report rows: %w", err)
	}

	rows := make([]models.AidRequestReportRow, 0, len(aidRequests))
	for _, aidRequest := range aidRequests {
		rows = append(rows, models.AidRequestReportRow{
			ID:              aidRequest.ID,
			PersonName:      aidRequest.Person.FullName,
			AidType:         aidRequest.AidType,
			Priority:        aidRequest.Priority,
			Title:           aidRequest.Title,
			RequestedAmount: aidRequest.RequestedAmount,
			ApprovedAmount:  aidRequest.ApprovedAmount,
			RequestDate:     aidRequest.RequestDate,
			Status:          aidRequest.Status,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetCareProvidedReportRows(
	filter models.ReportFilter,
) ([]models.CareProvidedReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.CareProvided{}).Where("is_deleted = FALSE")

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(description) LIKE ? OR LOWER(care_type) LIKE ? OR LOWER(provided_by) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	if strings.TrimSpace(filter.Type) != "" {
		query = query.Where("LOWER(care_type) = LOWER(?)", strings.TrimSpace(filter.Type))
	}

	query, err := applyDateRange(query, "provided_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count care provided report rows: %w", err)
	}

	var careRecords []models.CareProvided
	if err := query.
		Preload("Person").
		Order("provided_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&careRecords).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list care provided report rows: %w", err)
	}

	rows := make([]models.CareProvidedReportRow, 0, len(careRecords))
	for _, care := range careRecords {
		rows = append(rows, models.CareProvidedReportRow{
			ID:         care.ID,
			PersonName: care.Person.FullName,
			CareType:   care.CareType,
			ProvidedBy: care.ProvidedBy,
			Amount:     care.Amount,
			ProvidedAt: care.ProvidedAt,
			Status:     care.Status,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetLoanReportRows(
	filter models.ReportFilter,
) ([]models.LoanReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.Loan{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(purpose) LIKE ?",
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	query, err := applyDateRange(query, "created_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count loan report rows: %w", err)
	}

	var loans []models.Loan
	if err := query.
		Preload("Person").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&loans).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list loan report rows: %w", err)
	}

	rows := make([]models.LoanReportRow, 0, len(loans))
	for _, loan := range loans {
		rows = append(rows, models.LoanReportRow{
			ID:           loan.ID,
			PersonName:   loan.Person.FullName,
			LoanAmount:   loan.LoanAmount,
			InterestRate: loan.InterestRate,
			PeriodMonths: loan.DurationMonths,
			Status:       loan.Status,
			CreatedAt:    loan.CreatedAt,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetLoanRepaymentReportRows(
	filter models.ReportFilter,
) ([]models.LoanRepaymentReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.LoanRepayment{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(payment_reference) LIKE ? OR LOWER(notes) LIKE ?",
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(status) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	query, err := applyDateRange(query, "due_date", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count loan repayment report rows: %w", err)
	}

	var repayments []models.LoanRepayment
	if err := query.
		Preload("Loan").
		Preload("Loan.Person").
		Order("due_date DESC, installment_number ASC").
		Limit(pageSize).
		Offset(offset).
		Find(&repayments).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list loan repayment report rows: %w", err)
	}

	rows := make([]models.LoanRepaymentReportRow, 0, len(repayments))
	for _, repayment := range repayments {
		rows = append(rows, models.LoanRepaymentReportRow{
			ID:                repayment.ID,
			PersonName:        repayment.Loan.Person.FullName,
			InstallmentNumber: repayment.InstallmentNumber,
			DueDate:           repayment.DueDate,
			Amount:            repayment.Amount,
			PaidAmount:        repayment.PaidAmount,
			Status:            repayment.Status,
		})
	}

	return rows, total, nil
}

func (r *ReportRepository) GetRevenueReportRows(
	filter models.ReportFilter,
) ([]models.RevenueReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	query := r.db.Model(&models.RevenueRecord{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		query = query.Where(
			"LOWER(description) LIKE ? OR LOWER(category) LIKE ? OR LOWER(reference_no) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Type) != "" {
		query = query.Where("LOWER(record_type) = LOWER(?)", strings.TrimSpace(filter.Type))
	}

	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("LOWER(category) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	query, err := applyDateRange(query, "record_date", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count revenue report rows: %w", err)
	}

	var records []models.RevenueRecord
	if err := query.
		Order("record_date DESC, created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&records).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list revenue report rows: %w", err)
	}

	rows := make([]models.RevenueReportRow, 0, len(records))
	for _, record := range records {
		rows = append(rows, models.RevenueReportRow{
			ID:          record.ID,
			RecordType:  record.RecordType,
			Category:    record.Category,
			Amount:      record.Amount,
			Currency:    record.Currency,
			RecordDate:  record.RecordDate,
			Description: record.Description,
		})
	}

	return rows, total, nil
}

// GetAuditLogReportRows lists audit entries joined to their user names.
// The projection deliberately excludes any IP-address column: the audit
// table has none (migration 000002_drop_audit_ip) and must never gain one.
func (r *ReportRepository) GetAuditLogReportRows(
	filter models.ReportFilter,
) ([]models.AuditLogReportRow, int64, error) {
	_, pageSize, offset := normalizeReportPage(filter)

	filters := r.db.Model(&models.AuditLog{})

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		filters = filters.Where(
			"LOWER(action) LIKE ? OR LOWER(entity) LIKE ? OR LOWER(details) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		filters = filters.Where("LOWER(entity) = LOWER(?)", strings.TrimSpace(filter.Status))
	}

	if strings.TrimSpace(filter.Type) != "" {
		filters = filters.Where("LOWER(action) = LOWER(?)", strings.TrimSpace(filter.Type))
	}

	filters, err := applyDateRange(filters, "created_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := filters.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit log report rows: %w", err)
	}

	listQuery := r.db.
		Table("audit_logs").
		Select(
			"audit_logs.id, audit_logs.action, audit_logs.entity, audit_logs.entity_id, " +
				"audit_logs.details, audit_logs.old_value, audit_logs.new_value, " +
				"audit_logs.request_id, audit_logs.created_at, users.username AS username",
		).
		Joins("LEFT JOIN users ON users.id = audit_logs.user_id")

	// Re-apply the same parameterised conditions to the joined query.
	listQuery, err = applyDateRange(listQuery, "audit_logs.created_at", filter.From, filter.To)
	if err != nil {
		return nil, 0, err
	}

	if strings.TrimSpace(filter.Query) != "" {
		pattern := reportContains(filter.Query)
		listQuery = listQuery.Where(
			"LOWER(audit_logs.action) LIKE ? OR LOWER(audit_logs.entity) LIKE ? OR LOWER(audit_logs.details) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	if strings.TrimSpace(filter.Status) != "" {
		listQuery = listQuery.Where(
			"LOWER(audit_logs.entity) = LOWER(?)",
			strings.TrimSpace(filter.Status),
		)
	}

	if strings.TrimSpace(filter.Type) != "" {
		listQuery = listQuery.Where(
			"LOWER(audit_logs.action) = LOWER(?)",
			strings.TrimSpace(filter.Type),
		)
	}

	var rows []models.AuditLogReportRow
	if err := listQuery.
		Order("audit_logs.created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&rows).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list audit log report rows: %w", err)
	}

	return rows, total, nil
}
