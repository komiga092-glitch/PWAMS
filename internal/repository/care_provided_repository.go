package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrCareProvidedNotFound = errors.New("care provided record not found")

type CareProvidedRepository struct {
	db *gorm.DB
}

func NewCareProvidedRepository(db *gorm.DB) *CareProvidedRepository {
	return &CareProvidedRepository{
		db: db,
	}
}

func (r *CareProvidedRepository) Create(
	careProvided *models.CareProvided,
) error {
	if err := r.db.Create(careProvided).Error; err != nil {
		return fmt.Errorf("failed to create care provided record: %w", err)
	}

	return nil
}

func (r *CareProvidedRepository) FindByID(
	id uuid.UUID,
) (*models.CareProvided, error) {
	var careProvided models.CareProvided

	err := r.db.
		Preload("AidRequest").
		Preload("Person").
		Preload("CreatedBy").
		First(&careProvided, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCareProvidedNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"failed to find care provided record: %w",
			err,
		)
	}

	return &careProvided, nil
}

func (r *CareProvidedRepository) List(
	search string,
	status string,
	offset int,
	limit int,
) ([]models.CareProvided, int64, error) {
	var records []models.CareProvided
	var total int64

	// Search/status are applied to the SAME predicate set as the find query
	// (identical predicate before Count and before Find) so the filtered page
	// and its total always describe the same row set (QA CARE-003).
	query := r.db.Model(&models.CareProvided{})

	if strings.TrimSpace(search) != "" {
		searchValue := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"

		query = query.Where(
			`LOWER(CAST(id AS TEXT)) LIKE ?
			OR LOWER(CAST(aid_request_id AS TEXT)) LIKE ?
			OR LOWER(CAST(person_id AS TEXT)) LIKE ?
			OR LOWER(description) LIKE ?
			OR LOWER(provided_by) LIKE ?
			OR LOWER(care_type) LIKE ?`,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
		)
	}

	if strings.TrimSpace(status) != "" {
		query = query.Where(
			"LOWER(status) = LOWER(?)",
			strings.TrimSpace(status),
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to count care provided records: %w",
			err,
		)
	}

	err := query.
		Preload("AidRequest").
		Preload("Person").
		Preload("CreatedBy").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&records).Error

	if err != nil {
		return nil, 0, fmt.Errorf(
			"failed to list care provided records: %w",
			err,
		)
	}

	return records, total, nil
}

func (r *CareProvidedRepository) Update(
	careProvided *models.CareProvided,
) error {
	if err := r.db.Save(careProvided).Error; err != nil {
		return fmt.Errorf(
			"failed to update care provided record: %w",
			err,
		)
	}

	return nil
}

func (r *CareProvidedRepository) UpdateStatus(
	id uuid.UUID,
	status string,
) error {
	result := r.db.
		Model(&models.CareProvided{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf(
			"failed to update care provided status: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return ErrCareProvidedNotFound
	}

	return nil
}

func (r *CareProvidedRepository) Delete(
	id uuid.UUID,
) error {
	result := r.db.
		Delete(&models.CareProvided{}, "id = ?", id)

	if result.Error != nil {
		return fmt.Errorf(
			"failed to delete care provided record: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return ErrCareProvidedNotFound
	}

	return nil
}
