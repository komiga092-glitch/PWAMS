package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	CareProvidedStatusPending   = "Pending"
	CareProvidedStatusCompleted = "Completed"
	CareProvidedStatusCancelled = "Cancelled"
)

type CareProvided struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	AidRequestID uuid.UUID  `gorm:"type:uuid;not null;index" json:"aid_request_id"`
	AidRequest   AidRequest `gorm:"foreignKey:AidRequestID" json:"aid_request"`

	PersonID uuid.UUID `gorm:"type:uuid;not null;index" json:"person_id"`
	Person   Person    `gorm:"foreignKey:PersonID" json:"person"`

	Amount float64 `gorm:"type:numeric(12,2);not null" json:"amount"`

	Description string `gorm:"type:text" json:"description"`
	CareType    string `gorm:"size:50;not null;default:'other'" json:"care_type"`
	ProvidedBy  string `gorm:"size:150" json:"provided_by"`

	Status string `gorm:"size:20;not null;default:'Pending';index" json:"status"`

	ProvidedAt time.Time `gorm:"not null" json:"provided_at"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User      `gorm:"foreignKey:CreatedByID" json:"created_by"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Version   int        `gorm:"not null;default:1" json:"version"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid" json:"updated_by"`
	IsDeleted bool       `gorm:"not null;default:false;index" json:"is_deleted"`
	TenantID  *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id"`
}

func (CareProvided) TableName() string {
	return "care_provided"
}
