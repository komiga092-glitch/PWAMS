package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	RepaymentStatusPending   = "Pending"
	RepaymentStatusPaid      = "Paid"
	RepaymentStatusOverdue   = "Overdue"
	RepaymentStatusCancelled = "Cancelled"
)

type LoanRepayment struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	LoanID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uk_loan_repayment_installment" json:"loan_id"`
	Loan   Loan      `gorm:"foreignKey:LoanID" json:"loan,omitempty"`

	InstallmentNumber int `gorm:"not null;uniqueIndex:uk_loan_repayment_installment" json:"installment_number"`

	DueDate time.Time `gorm:"not null" json:"due_date"`

	Amount decimal.Decimal `gorm:"type:numeric(15,2);not null" json:"amount"`

	PaidAmount decimal.Decimal `gorm:"type:numeric(15,2);not null;default:0" json:"paid_amount"`

	PaidAt *time.Time `json:"paid_at,omitempty"`

	Status string `gorm:"type:varchar(30);not null;default:'Pending';index" json:"status"`

	PaymentReference string `gorm:"type:varchar(255)" json:"payment_reference,omitempty"`

	Notes string `gorm:"type:text" json:"notes,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Version   int        `gorm:"not null;default:1" json:"version"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid" json:"updated_by"`
	IsDeleted bool       `gorm:"not null;default:false;index" json:"is_deleted"`
	TenantID  *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id"`
}

type CreateLoanRepaymentRequest struct {
	LoanID            string          `json:"loan_id" binding:"required,uuid"`
	InstallmentNumber int             `json:"installment_number" binding:"required"`
	DueDate           string          `json:"due_date" binding:"required"`
	Amount            decimal.Decimal `json:"amount" binding:"required"`
	Notes             string          `json:"notes"`
}

type PayLoanRepaymentRequest struct {
	PaidAmount       decimal.Decimal `json:"paid_amount" binding:"required"`
	PaymentReference string          `json:"payment_reference"`
	Notes            string          `json:"notes"`
}

type LoanRepaymentListQuery struct {
	LoanID   string
	Status   string
	Page     int
	PageSize int
}
