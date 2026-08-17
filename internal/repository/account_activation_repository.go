package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

type AccountActivationRepository struct {
	db *gorm.DB
}

func NewAccountActivationRepository(db *gorm.DB) *AccountActivationRepository {
	return &AccountActivationRepository{
		db: db,
	}
}

func (r *AccountActivationRepository) Create(
	token *models.AccountActivationToken,
) error {
	return r.db.Create(token).Error
}

func (r *AccountActivationRepository) GetValidOTP(
	userID uuid.UUID,
	otp string,
) (*models.AccountActivationToken, error) {
	var token models.AccountActivationToken

	err := r.db.
		Where(
			"user_id = ? AND otp = ? AND verified = ? AND used_at IS NULL AND expires_at > ?",
			userID,
			otp,
			false,
			time.Now().UTC(),
		).
		Order("created_at DESC").
		First(&token).Error

	if err != nil {
		return nil, err
	}

	return &token, nil
}

func (r *AccountActivationRepository) MarkAsVerified(
	tokenID uuid.UUID,
) error {
	result := r.db.
		Model(&models.AccountActivationToken{}).
		Where("id = ?", tokenID).
		Update("verified", true)

	return result.Error
}

func (r *AccountActivationRepository) GetVerifiedOTP(
	userID uuid.UUID,
	otp string,
) (*models.AccountActivationToken, error) {
	var token models.AccountActivationToken

	err := r.db.
		Where(
			"user_id = ? AND otp = ? AND verified = ? AND used_at IS NULL AND expires_at > ?",
			userID,
			otp,
			true,
			time.Now().UTC(),
		).
		Order("created_at DESC").
		First(&token).Error

	if err != nil {
		return nil, err
	}

	return &token, nil
}

func (r *AccountActivationRepository) ConsumeOTP(
	tokenID uuid.UUID,
) error {
	now := time.Now().UTC()

	result := r.db.
		Model(&models.AccountActivationToken{}).
		Where(
			"id = ? AND verified = ? AND used_at IS NULL",
			tokenID,
			true,
		).
		Updates(map[string]interface{}{
			"used_at": now,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *AccountActivationRepository) DeleteExpired(
	userID uuid.UUID,
) error {
	return r.db.
		Where(
			"user_id = ? AND expires_at <= ?",
			userID,
			time.Now().UTC(),
		).
		Delete(&models.AccountActivationToken{}).
		Error
}
