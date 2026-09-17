package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var (
	ErrAdminDeletionRequestNotFound = errors.New("admin deletion request not found")
)

// AdminDeletionRequestRepository persists approval requests for
// deleting Admin accounts. Deleting an Admin requires approval by
// another authorized Admin; this repository is the minimal workflow
// store for that requirement.
type AdminDeletionRequestRepository struct {
	db *gorm.DB
}

func NewAdminDeletionRequestRepository(
	db *gorm.DB,
) *AdminDeletionRequestRepository {
	return &AdminDeletionRequestRepository{db: db}
}

// CreatePendingTx records a new Pending deletion request inside the
// given transaction. Any earlier Pending request for the same target
// is superseded (soft-deleted) so at most one live request exists per
// target Admin.
func (r *AdminDeletionRequestRepository) CreatePendingTx(
	tx *gorm.DB,
	targetUserID, requestedByID uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	request := &models.AdminDeletionRequest{
		TargetUserID:  targetUserID,
		RequestedByID: requestedByID,
		Status:        models.AdminDeletionStatusPending,
	}

	if err := tx.Create(request).Error; err != nil {
		return nil, fmt.Errorf("failed to create admin deletion request: %w", err)
	}

	return request, nil
}

// FindByID loads a deletion request by ID.
func (r *AdminDeletionRequestRepository) FindByID(
	id string,
) (*models.AdminDeletionRequest, error) {
	var request models.AdminDeletionRequest

	if err := r.db.First(&request, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAdminDeletionRequestNotFound
		}
		return nil, fmt.Errorf("failed to find admin deletion request: %w", err)
	}

	return &request, nil
}

// FindPendingByTargetIDTx returns the live Pending request for a
// target user inside the given transaction, if any.
func (r *AdminDeletionRequestRepository) FindPendingByTargetIDTx(
	tx *gorm.DB,
	targetUserID uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	var request models.AdminDeletionRequest

	err := tx.
		Where(
			"target_user_id = ? AND status = ?",
			targetUserID,
			models.AdminDeletionStatusPending,
		).
		Order("created_at DESC").
		First(&request).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find pending admin deletion request: %w", err)
	}

	return &request, nil
}

// FindApprovedByTargetIDTx returns the live APPROVED deletion request
// for a target user inside the given transaction, if any.
func (r *AdminDeletionRequestRepository) FindApprovedByTargetIDTx(
	tx *gorm.DB,
	targetUserID uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	var request models.AdminDeletionRequest

	err := tx.
		Where(
			"target_user_id = ? AND status = ?",
			targetUserID,
			models.AdminDeletionStatusApproved,
		).
		Order("decided_at DESC").
		First(&request).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find approved admin deletion request: %w", err)
	}

	return &request, nil
}

// MarkApprovedTx marks the request Approved inside the given
// transaction, recording the approver and decision time.
func (r *AdminDeletionRequestRepository) MarkApprovedTx(
	tx *gorm.DB,
	requestID, approvedByID uuid.UUID,
) error {
	now := time.Now().UTC()

	result := tx.
		Model(&models.AdminDeletionRequest{}).
		Where(
			"id = ? AND status = ?",
			requestID,
			models.AdminDeletionStatusPending,
		).
		Updates(map[string]interface{}{
			"status":         models.AdminDeletionStatusApproved,
			"approved_by_id": approvedByID,
			"decided_at":     now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to approve admin deletion request: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAdminDeletionRequestNotFound
	}

	return nil
}

// MarkRejectedTx marks the request Rejected inside the given
// transaction, recording the decider and decision time.
func (r *AdminDeletionRequestRepository) MarkRejectedTx(
	tx *gorm.DB,
	requestID, decidedByID uuid.UUID,
) error {
	now := time.Now().UTC()

	result := tx.
		Model(&models.AdminDeletionRequest{}).
		Where(
			"id = ? AND status = ?",
			requestID,
			models.AdminDeletionStatusPending,
		).
		Updates(map[string]interface{}{
			"status":         models.AdminDeletionStatusRejected,
			"approved_by_id": decidedByID,
			"decided_at":     now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to reject admin deletion request: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAdminDeletionRequestNotFound
	}

	return nil
}

// CreatePending records a new Pending deletion request outside an
// explicit transaction (convenience wrapper around CreatePendingTx).
func (r *AdminDeletionRequestRepository) CreatePending(
	targetUserID, requestedByID uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	return r.CreatePendingTx(r.db, targetUserID, requestedByID)
}

// FindPendingByTargetID returns the live Pending request for a target
// user outside an explicit transaction (wrapper around
// FindPendingByTargetIDTx).
func (r *AdminDeletionRequestRepository) FindPendingByTargetID(
	targetUserID uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	return r.FindPendingByTargetIDTx(r.db, targetUserID)
}

// MarkApproved marks the request Approved outside an explicit
// transaction (wrapper around MarkApprovedTx).
func (r *AdminDeletionRequestRepository) MarkApproved(
	requestID, approvedByID uuid.UUID,
) error {
	return r.MarkApprovedTx(r.db, requestID, approvedByID)
}

// MarkRejected marks the request Rejected outside an explicit
// transaction (wrapper around MarkRejectedTx).
func (r *AdminDeletionRequestRepository) MarkRejected(
	requestID, decidedByID uuid.UUID,
) error {
	return r.MarkRejectedTx(r.db, requestID, decidedByID)
}
