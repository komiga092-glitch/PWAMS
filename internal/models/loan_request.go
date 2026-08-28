package models

import "github.com/shopspring/decimal"

type CreateLoanRequest struct {
	PersonID       string          `json:"person_id" binding:"required,uuid"`
	LoanAmount     decimal.Decimal `json:"loan_amount" binding:"required"`
	InterestRate   decimal.Decimal `json:"interest_rate" binding:"required"`
	DurationMonths int             `json:"duration_months" binding:"required,gt=0"`
	Purpose        string          `json:"purpose"`
}

type UpdateLoanRequest struct {
	LoanAmount     decimal.Decimal `json:"loan_amount" binding:"required"`
	InterestRate   decimal.Decimal `json:"interest_rate" binding:"required"`
	DurationMonths int             `json:"duration_months" binding:"required,gt=0"`
	Purpose        string          `json:"purpose"`
}

type ReviewLoanRequest struct {
	Status string `json:"status" binding:"required"`

	ReviewNotes string `json:"review_notes"`
}

type LoanListQuery struct {
	Search   string `form:"search"`
	PersonID string `form:"person_id"`
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
