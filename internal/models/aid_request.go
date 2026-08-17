package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AidTypeMedical   = "Medical"
	AidTypeEducation = "Education"
	AidTypeFood      = "Food"
	AidTypeHousing   = "Housing"
	AidTypeClothing  = "Clothing"
	AidTypeEmergency = "Emergency"
	AidTypeOther     = "Other"

	AidPriorityLow      = "Low"
	AidPriorityMedium   = "Medium"
	AidPriorityHigh     = "High"
	AidPriorityCritical = "Critical"

	AidStatusPending     = "Pending"
	AidStatusUnderReview = "Under Review"
	AidStatusApproved    = "Approved"
	AidStatusRejected    = "Rejected"
	AidStatusCompleted   = "Completed"
	AidStatusCancelled   = "Cancelled"
)

type AidRequest struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	PersonID uuid.UUID `gorm:"type:uuid;not null;index" json:"person_id"`
	Person   Person    `gorm:"foreignKey:PersonID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"person"`

	AidType  string `gorm:"size:50;not null;index" json:"aid_type"`
	Priority string `gorm:"size:20;not null;default:Medium;index" json:"priority"`

	Title       string `gorm:"size:200;not null" json:"title"`
	Description string `gorm:"type:text;not null" json:"description"`

	RequestedAmount float64 `gorm:"type:numeric(14,2);default:0" json:"requested_amount"`
	ApprovedAmount  float64 `gorm:"type:numeric(14,2);default:0" json:"approved_amount"`
	Currency        string  `gorm:"size:10;default:LKR" json:"currency"`

	RequestDate time.Time  `gorm:"not null;index" json:"request_date"`
	NeededBy    *time.Time `gorm:"index" json:"needed_by,omitempty"`

	Status string `gorm:"size:30;not null;default:Pending;index" json:"status"`

	ReviewNotes string `gorm:"type:text" json:"review_notes"`

	ReviewedByID *uuid.UUID `gorm:"type:uuid;index" json:"reviewed_by_id,omitempty"`
	ReviewedBy   *User      `gorm:"foreignKey:ReviewedByID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"reviewed_by,omitempty"`

	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User      `gorm:"foreignKey:CreatedByID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"created_by"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (aidRequest *AidRequest) BeforeCreate(_ *gorm.DB) error {
	if aidRequest.ID == uuid.Nil {
		aidRequest.ID = uuid.New()
	}

	if aidRequest.Status == "" {
		aidRequest.Status = AidStatusPending
	}

	if aidRequest.Priority == "" {
		aidRequest.Priority = AidPriorityMedium
	}

	if aidRequest.Currency == "" {
		aidRequest.Currency = "LKR"
	}

	if aidRequest.RequestDate.IsZero() {
		aidRequest.RequestDate = time.Now().UTC()
	}

	return nil
}

func (AidRequest) TableName() string {
	return "aid_requests"
}
