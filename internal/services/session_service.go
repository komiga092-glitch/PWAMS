package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

const sessionDuration = 8 * time.Hour

type SessionService struct {
	sessionRepo *repository.SessionRepository
}

func NewSessionService(
	sessionRepo *repository.SessionRepository,
) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
	}
}

func (s *SessionService) CreateSession(
	userID uuid.UUID,
) (string, time.Time, error) {
	rawToken, err := generateSecureToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().Add(sessionDuration)

	session := models.Session{
		UserID:    userID,
		TokenHash: hashToken(rawToken),
		ExpiresAt: expiresAt,
	}

	if err := s.sessionRepo.Create(&session); err != nil {
		return "", time.Time{}, err
	}

	return rawToken, expiresAt, nil
}

func (s *SessionService) ValidateSession(
	rawToken string,
) (*models.User, error) {
	if rawToken == "" {
		return nil, repository.ErrSessionNotFound
	}

	session, err := s.sessionRepo.FindActiveByTokenHash(
		hashToken(rawToken),
	)
	if err != nil {
		return nil, err
	}

	if session.User.Status != models.UserStatusActive {
		return nil, fmt.Errorf("user account is not active")
	}

	return &session.User, nil
}

func (s *SessionService) RevokeSession(rawToken string) error {
	if rawToken == "" {
		return nil
	}

	return s.sessionRepo.RevokeByTokenHash(hashToken(rawToken))
}

func generateSecureToken() (string, error) {
	tokenBytes := make([]byte, 32)

	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(tokenBytes), nil
}

func hashToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}
