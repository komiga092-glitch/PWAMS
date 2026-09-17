package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Admin deletion request lifecycle states.
const (
	DeletionStatusPending   = "pending"
	DeletionStatusApproved  = "approved"
	DeletionStatusRejected  = "rejected"
	DeletionStatusCancelled = "cancelled"
	DeletionStatusExpired   = "expired"
)

// AdminDeletionRequest records a request to delete an Admin account.
// Deleting an Admin account is a two-step workflow (admin.delete.request /
// admin.delete.approve / admin.delete.reject): an actor holding the request
// permission raises the request, and a DIFFERENT actor (the service layer
// enforces approver != requester) approves or rejects it. Approval executes
// the deletion; rejection leaves the target untouched.
type AdminDeletionRequest struct {
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

	// Associations loaded for list/detail responses so the UI can display the
	// affected accounts without leaking any sensitive columns (password
	// material is omitted by the User JSON tags). PasswordHash is excluded by
	// its json:"-" tag.
	TargetUser  *User `gorm:"foreignKey:TargetUserID;references:ID" json:"target_user,omitempty"`
	Requester   *User `gorm:"foreignKey:RequesterID;references:ID" json:"requester,omitempty"`
	RespondedBy *User `gorm:"foreignKey:RespondedByID;references:ID" json:"responded_by,omitempty"`
}

func (d *AdminDeletionRequest) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.Status == "" {
		d.Status = DeletionStatusPending
	}
	if d.RequestedAt.IsZero() {
		d.RequestedAt = time.Now().UTC()
	}
	return nil
}

// CreateAdminDeletionRequest is the payload used to request the deletion of
// an Admin account.
type CreateAdminDeletionRequest struct {
	TargetUserID string `json:"target_user_id" binding:"required"`
	Reason       string `json:"reason"`
}

// RespondAdminDeletionRequest is the payload used to approve or reject an
// Admin deletion request.
type RespondAdminDeletionRequest struct {
	Response string `json:"response"`
}

// AdminDeletionRequestListQuery paginates the Admin deletion request listing.
type AdminDeletionRequestListQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}
