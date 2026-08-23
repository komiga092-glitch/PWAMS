package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RevenueTypeIncome  = "income"
	RevenueTypeExpense = "expense"

	RevenueCategoryDonations             = "Donations"
	RevenueCategoryLoanRepayments        = "Loan Repayments"
	RevenueCategoryGrants                = "Grants"
	RevenueCategoryAdministrativeExpense = "Administrative Expenses"
	RevenueCategoryWelfareExpense        = "Welfare Expenses"
)

type RevenueRecord struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	RecordType  string    `gorm:"size:20;not null;index" json:"record_type"`
	Category    string    `gorm:"size:50;not null;index" json:"category"`
	Amount      float64   `gorm:"type:numeric(15,2);not null" json:"amount"`
	Currency    string    `gorm:"size:10;not null;default:LKR" json:"currency"`
	RecordDate  time.Time `gorm:"not null;index" json:"record_date"`
	Description string    `gorm:"type:text" json:"description"`
	ReferenceNo string    `gorm:"size:100;uniqueIndex:idx_revenue_reference,where:reference_no <> ''" json:"reference_no"`

	CreatedByID uuid.UUID      `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User           `gorm:"foreignKey:CreatedByID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (record *RevenueRecord) BeforeCreate(_ *gorm.DB) error {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.Currency == "" {
		record.Currency = "LKR"
	}
	if record.RecordDate.IsZero() {
		record.RecordDate = time.Now().UTC()
	}
	return nil
}

func (RevenueRecord) TableName() string { return "revenue_records" }

type CreateRevenueRecordRequest struct {
	RecordType  string  `json:"record_type" binding:"required"`
	Category    string  `json:"category" binding:"required"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Currency    string  `json:"currency" binding:"omitempty,max=10"`
	RecordDate  string  `json:"record_date" binding:"required"`
	Description string  `json:"description" binding:"omitempty,max=1000"`
	ReferenceNo string  `json:"reference_no" binding:"omitempty,max=100"`
}

type UpdateRevenueRecordRequest struct {
	RecordType  string  `json:"record_type" binding:"required"`
	Category    string  `json:"category" binding:"required"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Currency    string  `json:"currency" binding:"omitempty,max=10"`
	RecordDate  string  `json:"record_date" binding:"required"`
	Description string  `json:"description" binding:"omitempty,max=1000"`
	ReferenceNo string  `json:"reference_no" binding:"omitempty,max=100"`
}

type RevenueListQuery struct {
	RecordType string `form:"record_type"`
	Category   string `form:"category"`
	FromDate   string `form:"from_date"`
	ToDate     string `form:"to_date"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

type RevenueSummary struct {
	Period   string  `json:"period"`
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Net      float64 `json:"net"`
}
