package repository

import (
	"errors"
	"fmt"
	"strings"

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
