package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RepaymentStatusPending   = "pending"
	RepaymentStatusPaid      = "paid"
	RepaymentStatusOverdue   = "overdue"
	RepaymentStatusCancelled = "cancelled"
)

type LoanRepayment struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	LoanID uuid.UUID `gorm:"type:uuid;not null;index" json:"loan_id"`
	Loan   Loan      `gorm:"foreignKey:LoanID" json:"loan,omitempty"`

	InstallmentNumber int `gorm:"not null" json:"installment_number"`

	DueDate time.Time `gorm:"not null" json:"due_date"`

	Amount float64 `gorm:"type:numeric(15,2);not null" json:"amount"`

	PaidAmount float64 `gorm:"type:numeric(15,2);not null;default:0" json:"paid_amount"`

	PaidAt *time.Time `json:"paid_at,omitempty"`

	Status string `gorm:"type:varchar(30);not null;default:'pending';index" json:"status"`

	PaymentReference string `gorm:"type:varchar(255)" json:"payment_reference,omitempty"`

	Notes string `gorm:"type:text" json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateLoanRepaymentRequest struct {
	LoanID            string  `json:"loan_id" binding:"required"`
	InstallmentNumber int     `json:"installment_number" binding:"required"`
	DueDate           string  `json:"due_date" binding:"required"`
	Amount            float64 `json:"amount" binding:"required,gt=0"`
	Notes             string  `json:"notes"`
}

type PayLoanRepaymentRequest struct {
	PaidAmount       float64 `json:"paid_amount" binding:"required,gt=0"`
	PaymentReference string  `json:"payment_reference"`
	Notes            string  `json:"notes"`
}

type LoanRepaymentListQuery struct {
	LoanID   string
	Status   string
	Page     int
	PageSize int
}
