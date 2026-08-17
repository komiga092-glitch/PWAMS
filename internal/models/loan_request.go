package models

type CreateLoanRequest struct {
	PersonID       string  `json:"person_id" binding:"required"`
	LoanAmount     float64 `json:"loan_amount" binding:"required,gt=0"`
	InterestRate   float64 `json:"interest_rate" binding:"gte=0"`
	DurationMonths int     `json:"duration_months" binding:"required,gt=0"`
	Purpose        string  `json:"purpose"`
}

type UpdateLoanRequest struct {
	LoanAmount     float64 `json:"loan_amount" binding:"required,gt=0"`
	InterestRate   float64 `json:"interest_rate" binding:"gte=0"`
	DurationMonths int     `json:"duration_months" binding:"required,gt=0"`
	Purpose        string  `json:"purpose"`
}

type ReviewLoanRequest struct {
	Status string `json:"status" binding:"required"`

	ReviewNotes string `json:"review_notes"`
}

type LoanListQuery struct {
	PersonID string `form:"person_id"`
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
