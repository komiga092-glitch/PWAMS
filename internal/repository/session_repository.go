package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{
		db: db,
	}
}

func (r *SessionRepository) Create(session *models.Session) error {
	if err := r.db.Create(session).Error; err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (r *SessionRepository) FindActiveByTokenHash(
	tokenHash string,
) (*models.Session, error) {
	var session models.Session

	err := r.db.
		Preload("User").
		Preload("User.Role").
		Where(
			"token_hash = ? AND revoked_at IS NULL AND expires_at > ?",
			tokenHash,
			time.Now(),
		).
		First(&session).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) RevokeByTokenHash(tokenHash string) error {
	now := time.Now()

	result := r.db.
		Model(&models.Session{}).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		Update("revoked_at", &now)

	if result.Error != nil {
		return fmt.Errorf("failed to revoke session: %w", result.Error)
	}

	return nil
}
