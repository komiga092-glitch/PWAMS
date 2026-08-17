package repository

import (
	"errors"
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

func (r *FileUploadRepository) FindByID(
	id string,
) (*models.FileUpload, error) {
	var file models.FileUpload

	err := r.db.
		Where("id = ?", id).
		First(&file).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrFileUploadNotFound
		}

		return nil, err
	}

	return &file, nil
}

func (r *FileUploadRepository) Delete(
	id string,
) error {
	result := r.db.
		Where("id = ?", id).
		Delete(&models.FileUpload{})

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
