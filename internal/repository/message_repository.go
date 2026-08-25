package repository

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

var ErrMessageNotFound = errors.New("message not found")

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(message *models.Message) error {
	if message == nil {
		return errors.New("message is nil")
	}

	return r.db.Create(message).Error
}

func (r *MessageRepository) FindByID(
	id uuid.UUID,
	userID uuid.UUID,
) (*models.Message, error) {
	var message models.Message

	err := r.db.
		Preload("Sender").
		Preload("Recipient").
		Where("id = ? AND is_deleted = FALSE", id).
		Where("sender_id = ? OR recipient_id = ?", userID, userID).
		First(&message).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMessageNotFound
	}

	if err != nil {
		return nil, err
	}

	return &message, nil
}

func (r *MessageRepository) ListInbox(
	userID uuid.UUID,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	return r.listForUser(userID, "recipient_id", page, pageSize)
}

func (r *MessageRepository) ListSent(
	userID uuid.UUID,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	return r.listForUser(userID, "sender_id", page, pageSize)
}

func (r *MessageRepository) ListUnreadInbox(
	userID uuid.UUID,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	var messages []models.Message
	var total int64
	page, pageSize = normalizeMessagePagination(page, pageSize)

	query := r.db.
		Model(&models.Message{}).
		Where("recipient_id = ? AND is_read = FALSE AND is_deleted = FALSE", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Sender").
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *MessageRepository) MarkAsRead(
	id uuid.UUID,
	userID uuid.UUID,
) error {
	result := r.db.
		Model(&models.Message{}).
		Where("id = ? AND recipient_id = ? AND is_deleted = FALSE", id, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrMessageNotFound
	}

	return nil
}

func (r *MessageRepository) SoftDelete(
	id uuid.UUID,
	userID uuid.UUID,
) error {
	result := r.db.
		Model(&models.Message{}).
		Where("id = ? AND is_deleted = FALSE", id).
		Where("sender_id = ? OR recipient_id = ?", userID, userID).
		Updates(map[string]interface{}{
			"is_deleted": true,
			"deleted_at": gorm.Expr("CURRENT_TIMESTAMP"),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrMessageNotFound
	}

	return nil
}

func (r *MessageRepository) CountUnread(userID uuid.UUID) (int64, error) {
	var count int64

	err := r.db.
		Model(&models.Message{}).
		Where("recipient_id = ? AND is_read = FALSE AND is_deleted = FALSE", userID).
		Count(&count).Error

	return count, err
}

func (r *MessageRepository) listForUser(
	userID uuid.UUID,
	column string,
	page int,
	pageSize int,
) ([]models.Message, int64, error) {
	var messages []models.Message
	var total int64
	page, pageSize = normalizeMessagePagination(page, pageSize)

	query := r.db.
		Model(&models.Message{}).
		Where(column+" = ? AND is_deleted = FALSE", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Sender").
		Preload("Recipient").
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func normalizeMessagePagination(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 20
	}

	if pageSize > 100 {
		pageSize = 100
	}

	return page, pageSize
}
