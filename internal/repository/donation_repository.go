package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrDonationNotFound = errors.New("donation not found")

type DonationRepository struct {
	db *gorm.DB
}

func NewDonationRepository(db *gorm.DB) *DonationRepository {
	return &DonationRepository{
		db: db,
	}
}

func (r *DonationRepository) ExistsByReferenceNo(
	referenceNo string,
) (bool, error) {
	referenceNo = strings.TrimSpace(referenceNo)

	if referenceNo == "" {
		return false, nil
	}

	var count int64

	if err := r.db.
		Model(&models.Donation{}).
		Where("UPPER(reference_no) = UPPER(?)", referenceNo).
		Count(&count).
		Error; err != nil {
		return false, fmt.Errorf(
			"failed to check donation reference: %w",
			err,
		)
	}

	return count > 0, nil
}

func (r *DonationRepository) Create(
	donation *models.Donation,
) error {
	if err := r.db.Create(donation).Error; err != nil {
		return fmt.Errorf("failed to create donation: %w", err)
	}

	return nil
}

func (r *DonationRepository) List(
	search string,
	donorID string,
	personID string,
	donationType string,
	status string,
	fromDate *time.Time,
	toDate *time.Time,
	page int,
	pageSize int,
) ([]models.Donation, int64, error) {
	var donations []models.Donation
	var total int64

	query := r.db.Model(&models.Donation{})

	if search != "" {
		searchValue := "%" +
			strings.ToLower(strings.TrimSpace(search)) +
			"%"

		query = query.Where(
			`LOWER(reference_no) LIKE ?
			OR LOWER(item_name) LIKE ?
			OR LOWER(description) LIKE ?`,
			searchValue,
			searchValue,
			searchValue,
		)
	}

	if donorID != "" {
		query = query.Where(
			"donor_id = ?",
			strings.TrimSpace(donorID),
		)
	}

	if personID != "" {
		query = query.Where(
			"person_id = ?",
			strings.TrimSpace(personID),
		)
	}

	if donationType != "" {
		query = query.Where(
			"LOWER(donation_type) = LOWER(?)",
			strings.TrimSpace(donationType),
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
			"donation_date >= ?",
			*fromDate,
		)
	}

	if toDate != nil {
		endOfDay := toDate.
			Add(24 * time.Hour).
			Add(-time.Nanosecond)

		query = query.Where(
			"donation_date <= ?",
			endOfDay,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to count donations: %w",
			err,
		)
	}

	offset := (page - 1) * pageSize

	if err := query.
		Preload("Donor").
		Preload("Person").
		Preload("CreatedBy").
		Order("donation_date DESC, created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&donations).
		Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to list donations: %w",
			err,
		)
	}

	return donations, total, nil
}
