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
	queryParams models.AidRequestListQuery,
) ([]models.AidRequest, int64, error) {
	var aidRequests []models.AidRequest
	var total int64

	query := r.db.Model(&models.AidRequest{})

	if queryParams.Search != "" {
		searchValue := "%" +
			strings.ToLower(strings.TrimSpace(queryParams.Search)) +
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

	personID := strings.TrimSpace(queryParams.PersonID)
	if personID != "" {
		query = query.Where(
			"person_id = ?",
			personID,
		)
	}

	aidType := strings.TrimSpace(queryParams.Type)
	if aidType != "" {
		query = query.Where(
			"LOWER(aid_type) = LOWER(?)",
			aidType,
		)
	}

	priority := strings.TrimSpace(queryParams.Priority)
	if priority != "" {
		query = query.Where(
			"LOWER(priority) = LOWER(?)",
			priority,
		)
	}

	status := strings.TrimSpace(queryParams.Status)
	if status != "" {
		query = query.Where(
			"LOWER(status) = LOWER(?)",
			status,
		)
	}

	if queryParams.FromDate != "" {
		parsedDate, err := time.Parse("2006-01-02", queryParams.FromDate)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse from date: %w", err)
		}

		query = query.Where(
			"request_date >= ?",
			parsedDate,
		)
	}

	if queryParams.ToDate != "" {
		parsedDate, err := time.Parse("2006-01-02", queryParams.ToDate)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse to date: %w", err)
		}

		endOfDay := parsedDate.Add(24 * time.Hour).Add(-time.Nanosecond)

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

	offset := (queryParams.Page - 1) * queryParams.PageSize

	if err := query.
		Preload("Person").
		Preload("CreatedBy").
		Preload("ReviewedBy").
		Order("request_date DESC, created_at DESC").
		Limit(queryParams.PageSize).
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
func (r *AidRequestRepository) Update(
	aidRequest *models.AidRequest,
) error {
	if err := r.db.Save(aidRequest).Error; err != nil {
		return fmt.Errorf(
			"failed to update aid request: %w",
			err,
		)
	}

	return nil
}
func (r *AidRequestRepository) UpdateReview(
	aidRequest *models.AidRequest,
) error {
	if err := r.db.Save(aidRequest).Error; err != nil {
		return fmt.Errorf(
			"failed to review aid request: %w",
			err,
		)
	}

	return nil
}
func (r *AidRequestRepository) SoftDelete(
	aidRequest *models.AidRequest,
) error {
	result := r.db.Delete(aidRequest)

	if result.Error != nil {
		return fmt.Errorf(
			"failed to delete aid request: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return ErrAidRequestNotFound
	}

	return nil
}
