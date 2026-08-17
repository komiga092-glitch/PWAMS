package services

import (
	"errors"
	"os"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var ErrInvalidFileUpload = errors.New("invalid file upload")

type FileUploadService struct {
	fileUploadRepo *repository.FileUploadRepository
}

func NewFileUploadService(
	fileUploadRepo *repository.FileUploadRepository,
) *FileUploadService {
	return &FileUploadService{
		fileUploadRepo: fileUploadRepo,
	}
}

func (s *FileUploadService) Create(
	file *models.FileUpload,
) error {
	if file == nil {
		return ErrInvalidFileUpload
	}

	if file.ID == uuid.Nil {
		file.ID = uuid.New()
	}

	if file.UserID == uuid.Nil {
		return ErrInvalidFileUpload
	}

	if file.OriginalName == "" ||
		file.StoredName == "" ||
		file.Path == "" ||
		file.ContentType == "" ||
		file.Size <= 0 {
		return ErrInvalidFileUpload
	}

	return s.fileUploadRepo.Create(file)
}

func (s *FileUploadService) GetByID(
	id string,
) (*models.FileUpload, error) {
	if id == "" {
		return nil, ErrInvalidFileUpload
	}

	return s.fileUploadRepo.FindByID(id)
}

func (s *FileUploadService) GetByIDForUser(
	id string,
	userID string,
) (*models.FileUpload, error) {
	if id == "" || userID == "" {
		return nil, ErrInvalidFileUpload
	}

	return s.fileUploadRepo.FindByIDAndUser(id, userID)
}

func (s *FileUploadService) Delete(id string) error {
	if id == "" {
		return ErrInvalidFileUpload
	}

	fileUpload, err := s.fileUploadRepo.FindByID(id)
	if err != nil {
		return err
	}

	if err := os.Remove(fileUpload.Path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return s.fileUploadRepo.Delete(id)
}
func (s *FileUploadService) DeleteForUser(
	id string,
	userID string,
) error {
	if id == "" || userID == "" {
		return ErrInvalidFileUpload
	}

	fileUpload, err := s.fileUploadRepo.FindByIDAndUser(
		id,
		userID,
	)
	if err != nil {
		return err
	}

	if err := os.Remove(fileUpload.Path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return s.fileUploadRepo.Delete(id)
}
