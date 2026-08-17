package repository

import (
	"errors"
	"fmt"

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
	offset int,
	limit int,
) ([]models.CareProvided, int64, error) {
	var records []models.CareProvided
	var total int64

	if err := r.db.
		Model(&models.CareProvided{}).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to count care provided records: %w",
			err,
		)
	}

	err := r.db.
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
