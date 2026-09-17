package models

// ── Report list queries (server-side filtering) ──────────────────────────

type UserReportQuery struct {
	Search   string `form:"search"`
	Role     string `form:"role"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type PersonReportQuery struct {
	Search   string `form:"search"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type StudentReportQuery struct {
	Search   string `form:"search"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type DonorReportQuery struct {
	Search   string `form:"search"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type DonationReportQuery struct {
	Search       string `form:"search"`
	DonorID      string `form:"donor_id"`
	Status       string `form:"status"`
	DonationType string `form:"donation_type"`
	DateFrom     string `form:"date_from"`
	DateTo       string `form:"date_to"`
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}

type AidRequestReportQuery struct {
	Search   string `form:"search"`
	Status   string `form:"status"`
	AidType  string `form:"aid_type"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type CareProvidedReportQuery struct {
	Search   string `form:"search"`
	CareType string `form:"care_type"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type LoanReportQuery struct {
	Search   string `form:"search"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type LoanRepaymentReportQuery struct {
	Search   string `form:"search"`
	LoanID   string `form:"loan_id"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type RevenueReportQuery struct {
	Search   string `form:"search"`
	Category string `form:"category"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type AuditActivityReportQuery struct {
	Search   string `form:"search"`
	Action   string `form:"action"`
	Entity   string `form:"entity"`
	UserID   string `form:"user_id"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type SystemAlertReportQuery struct {
	Search   string `form:"search"`
	Severity string `form:"severity"`
	Category string `form:"category"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// AccountStatusReportQuery drives the account-status report: the
// distribution of accounts per role and status plus the underlying account
// details. It shares the account filter set with the user report.
type AccountStatusReportQuery struct {
	Search   string `form:"search"`
	Role     string `form:"role"`
	Status   string `form:"status"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
