package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Deactivation request lifecycle states.
const (
	DeactivationStatusPending  = "pending"
	DeactivationStatusAccepted = "accepted"
	DeactivationStatusRejected = "rejected"
	DeactivationStatusExpired  = "expired"
)

// DeactivationRequest records a request to deactivate a Super Admin account.
// A Super Admin must not directly deactivate another Super Admin; instead a
// request is created, the target is notified, and the target either accepts
// or rejects it. Acceptance deactivates the target account.
type DeactivationRequest struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	TargetUserID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"target_user_id"`
	RequesterID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"requester_id"`
	Reason        string     `gorm:"type:text" json:"reason"`
	Status        string     `gorm:"size:20;not null;default:pending;index" json:"status"`
	RequestedAt   time.Time  `json:"requested_at"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
	RespondedByID *uuid.UUID `gorm:"type:uuid" json:"responded_by_id,omitempty"`
	Response      string     `gorm:"type:text" json:"response,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (d *DeactivationRequest) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.Status == "" {
		d.Status = DeactivationStatusPending
	}
	if d.RequestedAt.IsZero() {
		d.RequestedAt = time.Now().UTC()
	}
	return nil
}

// CreateDeactivationRequest is the payload used to request the deactivation
// of another Super Admin.
type CreateDeactivationRequest struct {
	TargetUserID string `json:"target_user_id" binding:"required"`
	Reason       string `json:"reason"`
}

// RespondDeactivationRequest is the payload used to accept or reject a
// deactivation request.
type RespondDeactivationRequest struct {
	Response string `json:"response"`
}

// DeactivationRequestListQuery paginates the deactivation request listing.
type DeactivationRequestListQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}
