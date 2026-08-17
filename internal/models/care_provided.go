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

	Status string `gorm:"size:20;not null;default:'Pending';index" json:"status"`

	ProvidedAt time.Time `gorm:"not null" json:"provided_at"`

	CreatedByID uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by_id"`
	CreatedBy   User      `gorm:"foreignKey:CreatedByID" json:"created_by"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
