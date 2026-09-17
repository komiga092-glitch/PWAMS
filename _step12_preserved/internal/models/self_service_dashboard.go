package models

import "github.com/shopspring/decimal"

// SelfServiceDashboardData holds the data for a Beneficiary or Student
// self-service dashboard. All data is scoped to the authenticated user.
type SelfServiceDashboardData struct {
	// Aid requests
	TotalAidRequests   int64 `json:"total_aid_requests"`
	PendingAidRequests int64 `json:"pending_aid_requests"`
	ApprovedAid        int64 `json:"approved_aid"`

	// Loans
	TotalLoans      int64 `json:"total_loans"`
	ActiveLoans     int64 `json:"active_loans"`
	PendingLoanApps int64 `json:"pending_loans"`

	// Financials
	TotalLoanAmount    decimal.Decimal `json:"total_loan_amount"`
	TotalRepaid        decimal.Decimal `json:"total_repaid"`
	OutstandingBalance decimal.Decimal `json:"outstanding_balance"`
	RepaymentPercent   float64         `json:"repayment_percent"`

	// Notifications
	UnreadNotifications int64 `json:"unread_notifications"`
}
