package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type DashboardReport struct {
	TotalUsers        int64 `json:"total_users"`
	TotalPersons      int64 `json:"total_persons"`
	TotalStudents     int64 `json:"total_students"`
	TotalDonors       int64 `json:"total_donors"`
	TotalDonations    int64 `json:"total_donations"`
	TotalAidRequests  int64 `json:"total_aid_requests"`
	TotalCareProvided int64 `json:"total_care_provided"`
}

type DonationReport struct {
	TotalDonations     int64            `json:"total_donations"`
	TotalAmount        float64          `json:"total_amount"`
	PendingDonations   int64            `json:"pending_donations"`
	ConfirmedDonations int64            `json:"confirmed_donations"`
	CancelledDonations int64            `json:"cancelled_donations"`
	ByType             map[string]int64 `json:"by_type"`
	ByStatus           map[string]int64 `json:"by_status"`
}

type AidRequestReport struct {
	TotalRequests     int64            `json:"total_requests"`
	PendingRequests   int64            `json:"pending_requests"`
	UnderReviewCount  int64            `json:"under_review_requests"`
	ApprovedRequests  int64            `json:"approved_requests"`
	RejectedRequests  int64            `json:"rejected_requests"`
	CompletedRequests int64            `json:"completed_requests"`
	CancelledRequests int64            `json:"cancelled_requests"`
	TotalRequested    float64          `json:"total_requested_amount"`
	TotalApproved     float64          `json:"total_approved_amount"`
	ByType            map[string]int64 `json:"by_type"`
	ByPriority        map[string]int64 `json:"by_priority"`
}

// UsersReport is the directory-oriented user report (report area "Users").
// AccountStatusReport below is the account-lifecycle view of the same table
// (report area "Account Status"); the two areas stay separate because the
// SRS lists user administration and account status as distinct reports.
type UsersReport struct {
	TotalUsers    int64            `json:"total_users"`
	ActiveUsers   int64            `json:"active_users"`
	DisabledUsers int64            `json:"disabled_users"`
	LockedUsers   int64            `json:"locked_users"`
	NewLast30Days int64            `json:"new_last_30_days"`
	ByRole        map[string]int64 `json:"by_role"`
	ByStatus      map[string]int64 `json:"by_status"`
}

type AccountStatusReport struct {
	TotalUsers    int64            `json:"total_users"`
	ActiveUsers   int64            `json:"active_users"`
	DisabledUsers int64            `json:"disabled_users"`
	LockedUsers   int64            `json:"locked_users"`
	LockedNow     int64            `json:"locked_now"`
	NeverLoggedIn int64            `json:"never_logged_in"`
	ByRole        map[string]int64 `json:"by_role"`
	ByStatus      map[string]int64 `json:"by_status"`
}

type PersonReport struct {
	TotalPersons    int64            `json:"total_persons"`
	ActivePersons   int64            `json:"active_persons"`
	InactivePersons int64            `json:"inactive_persons"`
	PendingPersons  int64            `json:"pending_persons"`
	ByStatus        map[string]int64 `json:"by_status"`
	ByGender        map[string]int64 `json:"by_gender"`
}

type StudentReport struct {
	TotalStudents    int64            `json:"total_students"`
	ActiveStudents   int64            `json:"active_students"`
	InactiveStudents int64            `json:"inactive_students"`
	PendingStudents  int64            `json:"pending_students"`
	RecentStudents   int64            `json:"recent_students"`
	ByStatus         map[string]int64 `json:"by_status"`
	ByGrade          map[string]int64 `json:"by_grade"`
	ByAcademicYear   map[string]int64 `json:"by_academic_year"`
}

type DonorReport struct {
	TotalDonors         int64            `json:"total_donors"`
	ActiveDonors        int64            `json:"active_donors"`
	InactiveDonors      int64            `json:"inactive_donors"`
	PendingDonors       int64            `json:"pending_donors"`
	TotalDonations      int64            `json:"total_donations"`
	TotalDonationAmount float64          `json:"total_donation_amount"`
	ByType              map[string]int64 `json:"by_type"`
	ByStatus            map[string]int64 `json:"by_status"`
}
type CareProvidedReport struct {
	TotalRecords     int64            `json:"total_records"`
	CompletedRecords int64            `json:"completed_records"`
	PendingRecords   int64            `json:"pending_records"`
	CancelledRecords int64            `json:"cancelled_records"`
	TotalAmount      float64          `json:"total_amount"`
	ByType           map[string]int64 `json:"by_type"`
	ByStatus         map[string]int64 `json:"by_status"`
}

type LoanReport struct {
	TotalLoans        int64            `json:"total_loans"`
	ActiveLoans       int64            `json:"active_loans"`
	PendingLoans      int64            `json:"pending_loans"`
	ApprovedLoans     int64            `json:"approved_loans"`
	CompletedLoans    int64            `json:"completed_loans"`
	RejectedLoans     int64            `json:"rejected_loans"`
	CancelledLoans    int64            `json:"cancelled_loans"`
	TotalDisbursed    float64          `json:"total_disbursed"`
	OutstandingAmount float64          `json:"outstanding_amount"`
	ByStatus          map[string]int64 `json:"by_status"`
}

type LoanRepaymentReport struct {
	TotalRepayments     int64            `json:"total_repayments"`
	PaidRepayments      int64            `json:"paid_repayments"`
	PendingRepayments   int64            `json:"pending_repayments"`
	OverdueRepayments   int64            `json:"overdue_repayments"`
	CancelledRepayments int64            `json:"cancelled_repayments"`
	TotalAmount         float64          `json:"total_amount"`
	TotalPaidAmount     float64          `json:"total_paid_amount"`
	OutstandingAmount   float64          `json:"outstanding_amount"`
	ByStatus            map[string]int64 `json:"by_status"`
}

type RevenueReport struct {
	TotalIncome   float64            `json:"total_income"`
	TotalExpenses float64            `json:"total_expenses"`
	Net           float64            `json:"net"`
	TotalRecords  int64              `json:"total_records"`
	ByCategory    map[string]float64 `json:"by_category"`
}

type AuditLogReport struct {
	TotalEntries int64            `json:"total_entries"`
	ByAction     map[string]int64 `json:"by_action"`
	ByEntity     map[string]int64 `json:"by_entity"`
}

// SystemAlertReport is derived from the audit trail: PWAMS has no dedicated
// alert table, so a "system alert" is an audit entry whose action belongs to
// the security/session set in SystemAlertActions. No client IP address is
// included anywhere in this report.
type SystemAlertReport struct {
	TotalAlerts    int64            `json:"total_alerts"`
	FailedLogins   int64            `json:"failed_logins"`
	LockedAccounts int64            `json:"locked_accounts"`
	DeniedAccess   int64            `json:"denied_access"`
	RecentAlerts   int64            `json:"recent_alerts"`
	ByAction       map[string]int64 `json:"by_action"`
	ByEntity       map[string]int64 `json:"by_entity"`
}

// SystemAlertActions lists the audit actions treated as system alerts. Keep in
// sync with the audit action strings written by the services.
var SystemAlertActions = []string{
	"LOGIN_FAILED",
	"ACCOUNT_LOCKED",
	"ACCESS_DENIED",
	"AUTHORIZATION_DENIED",
	"ACCOUNT_DISABLED",
	"PASSWORD_RESET",
	"SESSION_REVOKED",
}

// IsSystemAlertAction reports whether an audit action counts as a system alert.
func IsSystemAlertAction(action string) bool {
	for _, candidate := range SystemAlertActions {
		if candidate == action {
			return true
		}
	}
	return false
}

// ReportPagination is the shared pagination descriptor returned by every
// paginated report (HTML and JSON).
type ReportPagination struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalItems int64  `json:"total_items"`
	TotalPages int    `json:"total_pages"`
	HasPrev    bool   `json:"has_prev"`
	HasNext    bool   `json:"has_next"`
	PrevURL    string `json:"prev_url,omitempty"`
	NextURL    string `json:"next_url,omitempty"`
}

// ReportFilter is the shared, validated filter/pagination input for report
// detail queries. Every value is applied through parameterised GORM
// conditions — never interpolated into SQL.
type ReportFilter struct {
	From     string `form:"from"`
	To       string `form:"to"`
	Status   string `form:"status"`
	Priority string `form:"priority"`
	Type     string `form:"type"`
	Role     string `form:"role"`
	Grade    string `form:"grade"`
	Query    string `form:"q"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ────────────────────────────────────────────────────────────────────────────
// Detail rows for paginated report tables. These are read-only projections:
// every numeric or monetary field keeps its native Go type so templates can
// format consistently, and no internal database column name is exposed.
// ─────────────────────────────────────────────────────────────────────────────

type UserReportRow struct {
	ID          uuid.UUID  `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	RoleName    string     `json:"role"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type PersonReportRow struct {
	ID          uuid.UUID `json:"id"`
	FullName    string    `json:"full_name"`
	NICPassport string    `json:"nic_passport"`
	Gender      string    `json:"gender"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type StudentReportRow struct {
	ID           uuid.UUID `json:"id"`
	FullName     string    `json:"full_name"`
	StudentCode  string    `json:"student_code"`
	SchoolName   string    `json:"school_name"`
	Grade        string    `json:"grade"`
	AcademicYear int       `json:"academic_year"`
	Status       string    `json:"status"`
}

type DonorReportRow struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	DonorType string    `json:"donor_type"`
	Phone     string    `json:"phone"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type DonationReportRow struct {
	ID           uuid.UUID       `json:"id"`
	DonorName    string          `json:"donor_name"`
	DonationType string          `json:"donation_type"`
	ItemName     string          `json:"item_name"`
	Amount       decimal.Decimal `json:"amount"`
	Currency     string          `json:"currency"`
	DonationDate time.Time       `json:"donation_date"`
	Status       string          `json:"status"`
}

type AidRequestReportRow struct {
	ID              uuid.UUID       `json:"id"`
	PersonName      string          `json:"person_name"`
	AidType         string          `json:"aid_type"`
	Priority        string          `json:"priority"`
	Title           string          `json:"title"`
	RequestedAmount decimal.Decimal `json:"requested_amount"`
	ApprovedAmount  decimal.Decimal `json:"approved_amount"`
	RequestDate     time.Time       `json:"request_date"`
	Status          string          `json:"status"`
}

type CareProvidedReportRow struct {
	ID         uuid.UUID `json:"id"`
	PersonName string    `json:"person_name"`
	CareType   string    `json:"care_type"`
	ProvidedBy string    `json:"provided_by"`
	Amount     float64   `json:"amount"`
	ProvidedAt time.Time `json:"provided_at"`
	Status     string    `json:"status"`
}

type LoanReportRow struct {
	ID           uuid.UUID       `json:"id"`
	PersonName   string          `json:"person_name"`
	LoanAmount   decimal.Decimal `json:"loan_amount"`
	InterestRate decimal.Decimal `json:"interest_rate"`
	PeriodMonths int             `json:"duration_months"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
}

type LoanRepaymentReportRow struct {
	ID                uuid.UUID       `json:"id"`
	PersonName        string          `json:"person_name"`
	InstallmentNumber int             `json:"installment_number"`
	DueDate           time.Time       `json:"due_date"`
	Amount            decimal.Decimal `json:"amount"`
	PaidAmount        decimal.Decimal `json:"paid_amount"`
	Status            string          `json:"status"`
}

type RevenueReportRow struct {
	ID          uuid.UUID       `json:"id"`
	RecordType  string          `json:"record_type"`
	Category    string          `json:"category"`
	Amount      decimal.Decimal `json:"amount"`
	Currency    string          `json:"currency"`
	RecordDate  time.Time       `json:"record_date"`
	Description string          `json:"description"`
}

// AuditLogReportRow deliberately has no IP address field: the audit table and
// every audit report surface must stay free of client PII (migration
// 000002_drop_audit_ip drops the column and it must never be reintroduced).
type AuditLogReportRow struct {
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"user"`
	Action    string     `json:"action"`
	Entity    string     `json:"entity"`
	EntityID  *uuid.UUID `json:"entity_id,omitempty"`
	Details   string     `json:"details"`
	OldValue  string     `json:"old_value"`
	NewValue  string     `json:"new_value"`
	RequestID string     `json:"request_id"`
	CreatedAt time.Time  `json:"created_at"`
}
