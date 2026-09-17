package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrDeactivationRequestNotFound = errors.New("deactivation request not found")

type DeactivationRequestRepository struct {
	db *gorm.DB
}

func NewDeactivationRequestRepository(db *gorm.DB) *DeactivationRequestRepository {
	return &DeactivationRequestRepository{db: db}
}

func (r *DeactivationRequestRepository) Create(request *models.DeactivationRequest) error {
	if err := r.db.Create(request).Error; err != nil {
		return fmt.Errorf("failed to create deactivation request: %w", err)
	}
	return nil
}

func (r *DeactivationRequestRepository) FindByID(id uuid.UUID) (*models.DeactivationRequest, error) {
	var request models.DeactivationRequest

	err := r.db.Where("id = ?", id).First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDeactivationRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find deactivation request: %w", err)
	}

	return &request, nil
}

// ListForUser returns requests where the user is either the requester or the
// target, newest first. Pagination is applied server-side.
func (r *DeactivationRequestRepository) ListForUser(
	userID uuid.UUID,
	page, pageSize int,
) ([]models.DeactivationRequest, int64, error) {
	var requests []models.DeactivationRequest
	var total int64

	query := r.db.Model(&models.DeactivationRequest{}).
		Where("requester_id = ? OR target_user_id = ?", userID, userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count deactivation requests: %w", err)
	}

	offset := (page - 1) * pageSize
	if err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&requests).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list deactivation requests: %w", err)
	}

	return requests, total, nil
}

// FindPendingForTarget returns the first pending deactivation request directed
// at the given target user.
func (r *DeactivationRequestRepository) FindPendingForTarget(
	targetUserID uuid.UUID,
) (*models.DeactivationRequest, error) {
	var request models.DeactivationRequest

	err := r.db.
		Where("target_user_id = ? AND status = ?", targetUserID, models.DeactivationStatusPending).
		Order("created_at DESC").
		First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDeactivationRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find pending deactivation request: %w", err)
	}

	return &request, nil
}

// Respond resolves a pending request to accepted or rejected.
func (r *DeactivationRequestRepository) Respond(
	id uuid.UUID,
	status string,
	response string,
	respondedByID uuid.UUID,
) error {
	now := time.Now().UTC()
	result := r.db.Model(&models.DeactivationRequest{}).
		Where("id = ? AND status = ?", id, models.DeactivationStatusPending).
		Updates(map[string]interface{}{
			"status":          status,
			"response":        response,
			"responded_by_id": respondedByID,
			"responded_at":    now,
			"updated_at":      now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to respond to deactivation request: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrDeactivationRequestNotFound
	}

	return nil
}

// ExpireStale marks pending requests older than the cutoff date as expired.
func (r *DeactivationRequestRepository) ExpireStale(cutoff time.Time) (int64, error) {
	result := r.db.Model(&models.DeactivationRequest{}).
		Where("status = ? AND created_at < ?", models.DeactivationStatusPending, cutoff).
		Updates(map[string]interface{}{
			"status":     models.DeactivationStatusExpired,
			"updated_at": time.Now().UTC(),
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to expire stale deactivation requests: %w", result.Error)
	}

	return result.RowsAffected, nil
}
