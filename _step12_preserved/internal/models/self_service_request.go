package models

import "github.com/shopspring/decimal"

// SelfServiceAidRequest represents an aid request submitted by a
// Beneficiary or Student from their own dashboard. The person ID is
// never accepted from the client — it is derived server-side from the
// authenticated user's email.
type SelfServiceAidRequest struct {
	AidType         string          `form:"aid_type" json:"aid_type" binding:"required"`
	Title           string          `form:"title" json:"title" binding:"required,min=3,max=200"`
	Description     string          `form:"description" json:"description" binding:"required"`
	Priority        string          `form:"priority" json:"priority"`
	RequestedAmount decimal.Decimal `form:"requested_amount" json:"requested_amount"`
	Currency        string          `form:"currency" json:"currency"`
	NeededBy        string          `form:"needed_by" json:"needed_by"`
}

// SelfServiceLoanRequest represents a loan application submitted by a
// Beneficiary or Student from their own dashboard. The person ID is
// never accepted from the client — it is derived server-side.
type SelfServiceLoanRequest struct {
	LoanAmount     decimal.Decimal `form:"loan_amount" json:"loan_amount"`
	InterestRate   decimal.Decimal `form:"interest_rate" json:"interest_rate"`
	DurationMonths string          `form:"duration_months" json:"duration_months" binding:"required"`
	StartDate      string          `form:"start_date" json:"start_date"`
	DueDay         int             `form:"due_day" json:"due_day"`
	Purpose        string          `form:"purpose" json:"purpose" binding:"required"`
}
