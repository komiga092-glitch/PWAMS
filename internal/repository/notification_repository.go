package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrNotificationNotFound = errors.New("notification not found")

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

func (r *NotificationRepository) Create(
	notification *models.Notification,
) error {
	if notification == nil {
		return errors.New("notification is nil")
	}

	if notification.ID == uuid.Nil {
		notification.ID = uuid.New()
	}

	return r.db.Create(notification).Error
}

func (r *NotificationRepository) FindByID(
	id string,
) (*models.Notification, error) {
	notificationID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrNotificationNotFound
	}

	var notification models.Notification

	err = r.db.
		Where("id = ?", notificationID).
		First(&notification).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotificationNotFound
	}

	if err != nil {
		return nil, err
	}

	return &notification, nil
}

func (r *NotificationRepository) FindByUser(
	userID uuid.UUID,
) ([]models.Notification, error) {
	var notifications []models.Notification

	err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&notifications).Error

	if err != nil {
		return nil, err
	}

	return notifications, nil
}

func (r *NotificationRepository) MarkAsRead(
	id string,
	userID uuid.UUID,
) error {
	notificationID, err := uuid.Parse(id)
	if err != nil {
		return ErrNotificationNotFound
	}

	result := r.db.
		Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrNotificationNotFound
	}

	return nil
}

func (r *NotificationRepository) Delete(
	id string,
	userID uuid.UUID,
) error {
	notificationID, err := uuid.Parse(id)
	if err != nil {
		return ErrNotificationNotFound
	}

	result := r.db.
		Where("id = ? AND user_id = ?", notificationID, userID).
		Delete(&models.Notification{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrNotificationNotFound
	}

	return nil
}
