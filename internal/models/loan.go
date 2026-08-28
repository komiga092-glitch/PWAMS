package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	LoanStatusPending   = "Pending"
	LoanStatusApproved  = "Approved"
	LoanStatusRejected  = "Rejected"
	LoanStatusActive    = "Active"
	LoanStatusCompleted = "Completed"
	LoanStatusCancelled = "Cancelled"
)

type Loan struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	PersonID uuid.UUID `gorm:"type:uuid;not null;index" json:"person_id"`

	LoanAmount decimal.Decimal `gorm:"type:numeric(15,2);not null" json:"loan_amount"`

	InterestRate decimal.Decimal `gorm:"type:numeric(5,2);not null;default:0" json:"interest_rate"`

	DurationMonths int `gorm:"not null" json:"duration_months"`

	InstallmentAmount decimal.Decimal `gorm:"type:numeric(15,2);not null;default:0" json:"installment_amount"`

	Status string `gorm:"type:varchar(30);not null;default:'Pending';index" json:"status"`

	Purpose string `gorm:"type:text" json:"purpose"`

	ApprovedByID *uuid.UUID `gorm:"type:uuid" json:"approved_by_id,omitempty"`

	ApprovedAt *time.Time `json:"approved_at,omitempty"`

	DisbursedAt *time.Time `json:"disbursed_at,omitempty"`

	CompletedAt *time.Time `json:"completed_at,omitempty"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null" json:"created_by_id"`

	CreatedAt time.Time `json:"created_at"`

	UpdatedAt time.Time  `json:"updated_at"`
	Version   int        `gorm:"not null;default:1" json:"version"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid" json:"updated_by"`
	IsDeleted bool       `gorm:"not null;default:false;index" json:"is_deleted"`
	TenantID  *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id"`

	Person Person `gorm:"foreignKey:PersonID" json:"person,omitempty"`
}
