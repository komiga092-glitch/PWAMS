package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrDonorNotFound = errors.New("donor not found")

type DonorRepository struct {
	db *gorm.DB
}

func NewDonorRepository(db *gorm.DB) *DonorRepository {
	return &DonorRepository{
		db: db,
	}
}

func (r *DonorRepository) ExistsByIdentity(
	nicPassport string,
	registrationNumber string,
) (bool, error) {
	var count int64

	query := r.db.Model(&models.Donor{})

	nicPassport = strings.TrimSpace(nicPassport)
	registrationNumber = strings.TrimSpace(registrationNumber)

	if nicPassport != "" && registrationNumber != "" {
		query = query.Where(
			"UPPER(nic_passport) = UPPER(?) OR UPPER(registration_number) = UPPER(?)",
			nicPassport,
			registrationNumber,
		)
	} else if nicPassport != "" {
		query = query.Where(
			"UPPER(nic_passport) = UPPER(?)",
			nicPassport,
		)
	} else if registrationNumber != "" {
		query = query.Where(
			"UPPER(registration_number) = UPPER(?)",
			registrationNumber,
		)
	} else {
		return false, nil
	}

	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check existing donor: %w", err)
	}

	return count > 0, nil
}

func (r *DonorRepository) Create(donor *models.Donor) error {
	if err := r.db.Create(donor).Error; err != nil {
		return fmt.Errorf("failed to create donor: %w", err)
	}

	return nil
}

func (r *DonorRepository) List(
	search string,
	donorType string,
	status string,
	page int,
	pageSize int,
) ([]models.Donor, int64, error) {
	var donors []models.Donor
	var total int64

	query := r.db.Model(&models.Donor{})

	if search != "" {
		searchValue := "%" +
			strings.ToLower(strings.TrimSpace(search)) +
			"%"

		query = query.Where(
			`LOWER(name) LIKE ?
			OR LOWER(nic_passport) LIKE ?
			OR LOWER(organization_name) LIKE ?
			OR LOWER(registration_number) LIKE ?
			OR LOWER(phone) LIKE ?
			OR LOWER(email) LIKE ?`,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
		)
	}

	if donorType != "" {
		query = query.Where(
			"LOWER(donor_type) = LOWER(?)",
			strings.TrimSpace(donorType),
		)
	}

	if status != "" {
		query = query.Where(
			"LOWER(status) = LOWER(?)",
			strings.TrimSpace(status),
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to count donors: %w",
			err,
		)
	}

	offset := (page - 1) * pageSize

	if err := query.
		Preload("CreatedBy").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&donors).
		Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to list donors: %w",
			err,
		)
	}

	return donors, total, nil
}

func (r *DonorRepository) FindByID(id string) (*models.Donor, error) {
	var donor models.Donor

	err := r.db.
		Preload("CreatedBy").
		First(&donor, "id = ?", id).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDonorNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find donor by id: %w", err)
	}

	return &donor, nil
}

func (r *DonorRepository) ExistsByIdentityExceptID(
	nicPassport string,
	registrationNumber string,
	donorID string,
) (bool, error) {
	var count int64

	query := r.db.
		Model(&models.Donor{}).
		Where("id <> ?", donorID)

	nicPassport = strings.TrimSpace(nicPassport)
	registrationNumber = strings.TrimSpace(registrationNumber)

	switch {
	case nicPassport != "" && registrationNumber != "":
		query = query.Where(
			"UPPER(nic_passport) = UPPER(?) OR UPPER(registration_number) = UPPER(?)",
			nicPassport,
			registrationNumber,
		)

	case nicPassport != "":
		query = query.Where(
			"UPPER(nic_passport) = UPPER(?)",
			nicPassport,
		)

	case registrationNumber != "":
		query = query.Where(
			"UPPER(registration_number) = UPPER(?)",
			registrationNumber,
		)

	default:
		return false, nil
	}

	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf(
			"failed to check duplicate donor identity: %w",
			err,
		)
	}

	return count > 0, nil
}
func (r *DonorRepository) Update(donor *models.Donor) error {
	if err := r.db.Save(donor).Error; err != nil {
		return fmt.Errorf("failed to update donor: %w", err)
	}

	return nil
}
