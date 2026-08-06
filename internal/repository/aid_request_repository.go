package repository

import (
	"errors"
	"fmt"

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
