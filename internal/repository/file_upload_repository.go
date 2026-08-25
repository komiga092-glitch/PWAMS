package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"

	"gorm.io/gorm"
)

var ErrFileUploadNotFound = errors.New("file upload not found")

type FileUploadRepository struct {
	db *gorm.DB
}

func NewFileUploadRepository(db *gorm.DB) *FileUploadRepository {
	return &FileUploadRepository{
		db: db,
	}
}

func (r *FileUploadRepository) Create(
	file *models.FileUpload,
) error {
	return r.db.Create(file).Error
}

func (r *FileUploadRepository) DeleteForUser(
	id string,
	userID string,
) error {
	result := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.FileUpload{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrFileUploadNotFound
	}

	return nil
}

func (r *FileUploadRepository) ListForUser(
	userID string,
	page int,
	pageSize int,
) ([]models.FileUpload, int64, error) {
	var files []models.FileUpload
	var total int64

	query := r.db.
		Model(&models.FileUpload{}).
		Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&files).Error
	if err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

func (r *FileUploadRepository) RestoreForUser(
	id string,
	userID string,
) error {
	result := r.db.
		Unscoped().
		Model(&models.FileUpload{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("deleted_at", nil)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrFileUploadNotFound
	}

	return nil
}

func (r *FileUploadRepository) FindByIDAndUser(
	id string,
	userID string,
) (*models.FileUpload, error) {
	var file models.FileUpload

	err := r.db.
		Where("id = ? AND user_id = ?", id, userID).
		First(&file).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrFileUploadNotFound
		}

		return nil, err
	}

	return &file, nil
}

func (r *FileUploadRepository) FindByIDAndUserUnscoped(
	id string,
	userID string,
) (*models.FileUpload, error) {
	var file models.FileUpload

	err := r.db.
		Unscoped().
		Where("id = ? AND user_id = ?", id, userID).
		First(&file).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrFileUploadNotFound
		}

		return nil, err
	}

	return &file, nil
}

func (r *FileUploadRepository) FindByUserAndIdempotencyKey(
	userID uuid.UUID,
	idempotencyKey string,
) (*models.FileUpload, error) {
	var file models.FileUpload

	err := r.db.
		Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).
		First(&file).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrFileUploadNotFound
		}

		return nil, err
	}

	return &file, nil
}

func (r *FileUploadRepository) ListAllUnscoped() ([]models.FileUpload, error) {
	var files []models.FileUpload
	if err := r.db.Unscoped().Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}
