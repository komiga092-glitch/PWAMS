package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

type PasswordResetRepository struct {
	db *gorm.DB
}

func NewPasswordResetRepository(db *gorm.DB) *PasswordResetRepository {
	return &PasswordResetRepository{
		db: db,
	}
}

func (r *PasswordResetRepository) Create(
	token *models.PasswordResetToken,
) error {
	return r.db.Create(token).Error
}

func (r *PasswordResetRepository) GetValidOTP(
	userID uuid.UUID,
	otp string,
) (*models.PasswordResetToken, error) {
	var token models.PasswordResetToken

	err := r.db.
		Where(
			"user_id = ? AND otp = ? AND verified = ? AND expires_at > ?",
			userID,
			otp,
			false,
			time.Now(),
		).
		Order("created_at DESC").
		First(&token).Error

	if err != nil {
		return nil, err
	}

	return &token, nil
}
func (r *PasswordResetRepository) GetVerifiedOTP(
	userID uuid.UUID,
	otp string,
) (*models.PasswordResetToken, error) {
	var token models.PasswordResetToken

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

func (r *PasswordResetRepository) ConsumeOTP(
	tokenID uuid.UUID,
) error {
	now := time.Now().UTC()

	result := r.db.
		Model(&models.PasswordResetToken{}).
		Where("id = ? AND verified = ? AND used_at IS NULL", tokenID, true).
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

func (r *PasswordResetRepository) MarkAsVerified(
	tokenID uuid.UUID,
) error {
	return r.db.
		Model(&models.PasswordResetToken{}).
		Where("id = ?", tokenID).
		Update("verified", true).
		Error
}

func (r *PasswordResetRepository) DeleteExpired(
	userID uuid.UUID,
) error {
	return r.db.
		Where(
			"user_id = ? AND expires_at <= ?",
			userID,
			time.Now(),
		).
		Delete(&models.PasswordResetToken{}).
		Error
}
