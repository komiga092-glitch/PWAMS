package services

import (
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidNotification = errors.New("invalid notification")
)

type NotificationService struct {
	notificationRepo *repository.NotificationRepository
}

func NewNotificationService(
	notificationRepo *repository.NotificationRepository,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
	}
}

func (s *NotificationService) Create(
	notification *models.Notification,
) error {
	if notification == nil {
		return ErrInvalidNotification
	}

	if notification.UserID == uuid.Nil {
		return ErrInvalidNotification
	}

	notification.Title = strings.TrimSpace(notification.Title)
	notification.Message = strings.TrimSpace(notification.Message)
	notification.Type = strings.TrimSpace(notification.Type)

	if notification.Title == "" ||
		notification.Message == "" ||
		notification.Type == "" {
		return ErrInvalidNotification
	}

	if notification.ID == uuid.Nil {
		notification.ID = uuid.New()
	}

	return s.notificationRepo.Create(notification)
}

func (s *NotificationService) GetByID(
	id string,
) (*models.Notification, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrInvalidNotification
	}

	return s.notificationRepo.FindByID(id)
}

func (s *NotificationService) ListForUser(
	userID uuid.UUID,
) ([]models.Notification, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidNotification
	}

	return s.notificationRepo.FindByUser(userID)
}

func (s *NotificationService) MarkAsRead(
	id string,
	userID uuid.UUID,
) error {
	if strings.TrimSpace(id) == "" || userID == uuid.Nil {
		return ErrInvalidNotification
	}

	return s.notificationRepo.MarkAsRead(id, userID)
}

func (s *NotificationService) Delete(
	id string,
	userID uuid.UUID,
) error {
	if strings.TrimSpace(id) == "" || userID == uuid.Nil {
		return ErrInvalidNotification
	}

	return s.notificationRepo.Delete(id, userID)
}
