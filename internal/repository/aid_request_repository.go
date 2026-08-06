package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrAidRequestNotFound = errors.New("aid request not found")

type AidRequestRepository struct {
	db *gorm.DB
}

func NewAidRequestRepository(db *gorm.DB) *AidRequestRepository {
	return &AidRequestRepository{
		db: db,
	}
}

func (r *AidRequestRepository) Create(
	aidRequest *models.AidRequest,
) error {
	if err := r.db.Create(aidRequest).Error; err != nil {
		return fmt.Errorf("failed to create aid request: %w", err)
	}

	return nil
}

func (r *AidRequestRepository) List(
	search string,
	personID string,
	aidType string,
	priority string,
	status string,
	fromDate *time.Time,
	toDate *time.Time,
	page int,
	pageSize int,
) ([]models.AidRequest, int64, error) {
	var aidRequests []models.AidRequest
	var total int64

	query := r.db.Model(&models.AidRequest{})

	if search != "" {
		searchValue := "%" +
			strings.ToLower(strings.TrimSpace(search)) +
			"%"

		query = query.Where(
			`LOWER(title) LIKE ?
			OR LOWER(description) LIKE ?
			OR LOWER(review_notes) LIKE ?`,
			searchValue,
			searchValue,
			searchValue,
		)
	}

	if personID != "" {
		query = query.Where(
			"person_id = ?",
			strings.TrimSpace(personID),
		)
	}

	if aidType != "" {
		query = query.Where(
			"LOWER(aid_type) = LOWER(?)",
			strings.TrimSpace(aidType),
		)
	}

	if priority != "" {
		query = query.Where(
			"LOWER(priority) = LOWER(?)",
			strings.TrimSpace(priority),
		)
	}

	if status != "" {
		query = query.Where(
			"LOWER(status) = LOWER(?)",
			strings.TrimSpace(status),
		)
	}

	if fromDate != nil {
		query = query.Where(
			"request_date >= ?",
			*fromDate,
		)
	}

	if toDate != nil {
		endOfDay := toDate.
			Add(24 * time.Hour).
			Add(-time.Nanosecond)

		query = query.Where(
			"request_date <= ?",
			endOfDay,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to count aid requests: %w",
			err,
		)
	}

	offset := (page - 1) * pageSize

	if err := query.
		Preload("Person").
		Preload("CreatedBy").
		Preload("ReviewedBy").
		Order("request_date DESC, created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&aidRequests).
		Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to list aid requests: %w",
			err,
		)
	}

	return aidRequests, total, nil
}
func (r *AidRequestRepository) FindByID(
	id string,
) (*models.AidRequest, error) {
	var aidRequest models.AidRequest

	err := r.db.
		Preload("Person").
		Preload("CreatedBy").
		Preload("ReviewedBy").
		First(&aidRequest, "id = ?", id).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAidRequestNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"failed to find aid request by id: %w",
			err,
		)
	}

	return &aidRequest, nil
}
