package models

// ReportPagination holds pagination metadata for report responses.
type ReportPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// ── Report result wrappers (KPI + rows + pagination) ─────────────────────

type UserReportResult struct {
	Total      int64            `json:"total"`
	Active     int64            `json:"active"`
	Disabled   int64            `json:"disabled"`
	Locked     int64            `json:"locked"`
	Rows       []UserReportRow  `json:"rows"`
	Pagination ReportPagination `json:"pagination"`
}

type PersonReportResult struct {
	Total      int64             `json:"total"`
	Active     int64             `json:"active"`
	Inactive   int64             `json:"inactive"`
	Rows       []PersonReportRow `json:"rows"`
	Pagination ReportPagination  `json:"pagination"`
}

type StudentReportResult struct {
	Total      int64              `json:"total"`
	Active     int64              `json:"active"`
	Inactive   int64              `json:"inactive"`
	Rows       []StudentReportRow `json:"rows"`
	Pagination ReportPagination   `json:"pagination"`
}

type DonorReportResult struct {
	Total      int64            `json:"total"`
	Active     int64            `json:"active"`
	Inactive   int64            `json:"inactive"`
	Rows       []DonorReportRow `json:"rows"`
	Pagination ReportPagination `json:"pagination"`
}

type DonationReportResult struct {
	TotalDonations int64               `json:"total_donations"`
	TotalAmount    float64             `json:"total_amount"`
	Completed      int64               `json:"completed"`
	Pending        int64               `json:"pending"`
	Rows           []DonationReportRow `json:"rows"`
	Pagination     ReportPagination    `json:"pagination"`
}

type AidRequestReportResult struct {
	Total      int64                 `json:"total"`
	Pending    int64                 `json:"pending"`
	Approved   int64                 `json:"approved"`
	Rejected   int64                 `json:"rejected"`
	Cancelled  int64                 `json:"cancelled"`
	Rows       []AidRequestReportRow `json:"rows"`
	Pagination ReportPagination      `json:"pagination"`
}

type CareProvidedReportResult struct {
	Total      int64                   `json:"total"`
	Active     int64                   `json:"active"`
	Completed  int64                   `json:"completed"`
	Rows       []CareProvidedReportRow `json:"rows"`
	Pagination ReportPagination        `json:"pagination"`
}

type LoanReportResult struct {
	TotalLoans     int64            `json:"total_loans"`
	Active         int64            `json:"active"`
	Completed      int64            `json:"completed"`
	TotalPrincipal float64          `json:"total_principal"`
	TotalInterest  float64          `json:"total_interest"`
	TotalRepayable float64          `json:"total_repayable"`
	TotalRepaid    float64          `json:"total_repaid"`
	Rows           []LoanReportRow  `json:"rows"`
	Pagination     ReportPagination `json:"pagination"`
}

type LoanRepaymentReportResult struct {
	Total       int64                    `json:"total"`
	TotalAmount float64                  `json:"total_amount"`
	Collected   float64                  `json:"collected"`
	Overdue     int64                    `json:"overdue"`
	Rows        []LoanRepaymentReportRow `json:"rows"`
	Pagination  ReportPagination         `json:"pagination"`
}

type RevenueReportResult struct {
	TotalIncome   float64            `json:"total_income"`
	TotalExpenses float64            `json:"total_expenses"`
	Net           float64            `json:"net"`
	Rows          []RevenueReportRow `json:"rows"`
	Pagination    ReportPagination   `json:"pagination"`
}

type AuditActivityReportResult struct {
	Total      int64                    `json:"total"`
	Creates    int64                    `json:"creates"`
	Updates    int64                    `json:"updates"`
	Deletes    int64                    `json:"deletes"`
	Security   int64                    `json:"security"`
	Rows       []AuditActivityReportRow `json:"rows"`
	Pagination ReportPagination         `json:"pagination"`
}

type SystemAlertReportResult struct {
	Total      int64                  `json:"total"`
	Critical   int64                  `json:"critical"`
	Errors     int64                  `json:"errors"`
	Warnings   int64                  `json:"warnings"`
	Unresolved int64                  `json:"unresolved"`
	Rows       []SystemAlertReportRow `json:"rows"`
	Pagination ReportPagination       `json:"pagination"`
}

// AccountStatusDistributionRow is one per-role line of the account-status
// distribution table (role × status counts). The row set is bounded by the
// number of roles, so it is fetched without pagination.
type AccountStatusDistributionRow struct {
	Role     string `json:"role"`
	Active   int64  `json:"active"`
	Disabled int64  `json:"disabled"`
	Locked   int64  `json:"locked"`
	Total    int64  `json:"total"`
}

// AccountStatusReportResult combines the account KPIs, the per-role
// distribution table and the paginated account detail rows.
type AccountStatusReportResult struct {
	Total        int64                          `json:"total"`
	Active       int64                          `json:"active"`
	Disabled     int64                          `json:"disabled"`
	Locked       int64                          `json:"locked"`
	Distribution []AccountStatusDistributionRow `json:"distribution"`
	Rows         []AccountStatusReportRow       `json:"rows"`
	Pagination   ReportPagination               `json:"pagination"`
}
