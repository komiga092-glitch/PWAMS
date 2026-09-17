package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAdminDeletionRequestNotFound = errors.New("admin deletion request not found")

// AdminDeletionRequestRepository persists the Admin account deletion request
// workflow records (request / approve / reject).
type AdminDeletionRequestRepository struct {
	db *gorm.DB
}

func NewAdminDeletionRequestRepository(db *gorm.DB) *AdminDeletionRequestRepository {
	return &AdminDeletionRequestRepository{db: db}
}

func (r *AdminDeletionRequestRepository) Create(request *models.AdminDeletionRequest) error {
	if err := r.db.Create(request).Error; err != nil {
		return fmt.Errorf("failed to create admin deletion request: %w", err)
	}
	return nil
}

func (r *AdminDeletionRequestRepository) FindByID(id uuid.UUID) (*models.AdminDeletionRequest, error) {
	var request models.AdminDeletionRequest

	err := r.db.Where("id = ?", id).First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAdminDeletionRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find admin deletion request: %w", err)
	}

	return &request, nil
}

// FindPendingForTarget returns the first pending deletion request directed at
// the given target user.
func (r *AdminDeletionRequestRepository) FindPendingForTarget(
	targetUserID uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	var request models.AdminDeletionRequest

	err := r.db.
		Where("target_user_id = ? AND status = ?", targetUserID, models.DeletionStatusPending).
		Order("created_at DESC").
		First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAdminDeletionRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find pending admin deletion request: %w", err)
	}

	return &request, nil
}

// ListPending returns pending deletion requests, newest first, paginated.
// The target and requester accounts are preloaded (without any sensitive
// columns) so the UI can render the affected Admin accounts directly.
func (r *AdminDeletionRequestRepository) ListPending(
	page, pageSize int,
) ([]models.AdminDeletionRequest, int64, error) {
	var requests []models.AdminDeletionRequest
	var total int64

	query := r.db.Model(&models.AdminDeletionRequest{}).
		Where("status = ?", models.DeletionStatusPending)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count admin deletion requests: %w", err)
	}

	offset := (page - 1) * pageSize
	if err := query.
		Preload("TargetUser.Role").
		Preload("Requester.Role").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&requests).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list admin deletion requests: %w", err)
	}

	return requests, total, nil
}

// ListForUser returns requests where the user is either the requester or the
// target, newest first. Pagination is applied server-side.
func (r *AdminDeletionRequestRepository) ListForUser(
	userID uuid.UUID,
	page, pageSize int,
) ([]models.AdminDeletionRequest, int64, error) {
	var requests []models.AdminDeletionRequest
	var total int64

	query := r.db.Model(&models.AdminDeletionRequest{}).
		Where("requester_id = ? OR target_user_id = ?", userID, userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count admin deletion requests: %w", err)
	}

	offset := (page - 1) * pageSize
	if err := query.
		Preload("TargetUser.Role").
		Preload("Requester.Role").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&requests).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list admin deletion requests: %w", err)
	}

	return requests, total, nil
}

// Transaction runs fn inside a database transaction on the underlying
// connection, so the workflow service can execute row-locked reads and all
// of the resolution writes atomically.
func (r *AdminDeletionRequestRepository) Transaction(
	fn func(tx *gorm.DB) error,
) error {
	return r.db.Transaction(fn)
}

// FindByIDForUpdate loads a request with a row-level lock (SELECT ... FOR
// UPDATE) on the given transaction so concurrent approvals serialise on the
// request row instead of racing through the status check.
func (r *AdminDeletionRequestRepository) FindByIDForUpdate(
	tx *gorm.DB,
	id uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	var request models.AdminDeletionRequest

	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAdminDeletionRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lock admin deletion request: %w", err)
	}

	return &request, nil
}

// Respond resolves a pending request to approved or rejected.
func (r *AdminDeletionRequestRepository) Respond(
	id uuid.UUID,
	status string,
	response string,
	respondedByID uuid.UUID,
) error {
	return r.RespondTx(r.db, id, status, response, respondedByID)
}

// RespondTx is Respond running on the supplied transaction so the status
// flip is part of the same atomic unit as the account deactivation.
func (r *AdminDeletionRequestRepository) RespondTx(
	tx *gorm.DB,
	id uuid.UUID,
	status string,
	response string,
	respondedByID uuid.UUID,
) error {
	now := time.Now().UTC()
	result := tx.Model(&models.AdminDeletionRequest{}).
		Where("id = ? AND status = ?", id, models.DeletionStatusPending).
		Updates(map[string]interface{}{
			"status":          status,
			"response":        response,
			"responded_by_id": respondedByID,
			"responded_at":    now,
			"updated_at":      now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to respond to admin deletion request: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAdminDeletionRequestNotFound
	}

	return nil
}

// ExpireStale marks pending requests older than the cutoff date as expired.
func (r *AdminDeletionRequestRepository) ExpireStale(cutoff time.Time) (int64, error) {
	result := r.db.Model(&models.AdminDeletionRequest{}).
		Where("status = ? AND created_at < ?", models.DeletionStatusPending, cutoff).
		Updates(map[string]interface{}{
			"status":     models.DeletionStatusExpired,
			"updated_at": time.Now().UTC(),
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to expire stale admin deletion requests: %w", result.Error)
	}

	return result.RowsAffected, nil
}
