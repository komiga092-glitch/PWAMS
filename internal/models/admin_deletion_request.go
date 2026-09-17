package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Admin deletion request lifecycle states. Status values are stored
// with a capital first letter to match the application's domain-status
// convention (e.g. UserStatusActive = "Active").
const (
	AdminDeletionStatusPending  = "Pending"
	AdminDeletionStatusApproved = "Approved"
	AdminDeletionStatusRejected = "Rejected"
)

// AdminDeletionRequest records a request to delete an Admin account.
// Deleting an Admin account is a two-step workflow: an actor holding the
// request permission raises the request, and a DIFFERENT actor (the
// service layer enforces approver != requester) approves or rejects it.
// Approval executes the deletion; rejection leaves the target untouched.
type AdminDeletionRequest struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TargetUserID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"target_user_id"`
	RequestedByID uuid.UUID      `gorm:"type:uuid;not null;index" json:"requested_by_id"`
	Status        string         `gorm:"size:20;not null;default:Pending;index" json:"status"`
	ApprovedByID  *uuid.UUID     `gorm:"type:uuid" json:"approved_by_id,omitempty"`
	DecidedAt     *time.Time     `json:"decided_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (d *AdminDeletionRequest) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.Status == "" {
		d.Status = AdminDeletionStatusPending
	}
	return nil
}

// TableName returns the table name for the AdminDeletionRequest model.
func (AdminDeletionRequest) TableName() string {
	return "admin_deletion_requests"
}
