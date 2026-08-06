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
