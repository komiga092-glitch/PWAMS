package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	LoanStatusPending   = "pending"
	LoanStatusApproved  = "approved"
	LoanStatusRejected  = "rejected"
	LoanStatusActive    = "active"
	LoanStatusCompleted = "completed"
	LoanStatusCancelled = "cancelled"
)

type Loan struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	PersonID uuid.UUID `gorm:"type:uuid;not null;index" json:"person_id"`

	LoanAmount float64 `gorm:"type:numeric(15,2);not null" json:"loan_amount"`

	InterestRate float64 `gorm:"type:numeric(5,2);not null;default:0" json:"interest_rate"`

	DurationMonths int `gorm:"not null" json:"duration_months"`

	InstallmentAmount float64 `gorm:"type:numeric(15,2);not null;default:0" json:"installment_amount"`

	Status string `gorm:"type:varchar(30);not null;default:'pending';index" json:"status"`

	Purpose string `gorm:"type:text" json:"purpose"`

	ApprovedByID *uuid.UUID `gorm:"type:uuid" json:"approved_by_id,omitempty"`

	ApprovedAt *time.Time `json:"approved_at,omitempty"`

	DisbursedAt *time.Time `json:"disbursed_at,omitempty"`

	CompletedAt *time.Time `json:"completed_at,omitempty"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null" json:"created_by_id"`

	CreatedAt time.Time `json:"created_at"`

	UpdatedAt time.Time `json:"updated_at"`

	Person Person `gorm:"foreignKey:PersonID" json:"person,omitempty"`
}
