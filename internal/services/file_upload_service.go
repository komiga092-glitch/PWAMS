package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidFileUpload        = errors.New("invalid file upload")
	ErrFileUploadAlreadyDeleted = errors.New("file upload is already deleted")
)

type FileUploadService struct {
	fileUploadRepo *repository.FileUploadRepository
}

type FileReconciliation struct {
	MissingFiles  []FileReconciliationItem `json:"missing_files"`
	OrphanedFiles []string                 `json:"orphaned_files"`
}

type FileReconciliationItem struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
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

func (s *FileUploadService) GetByIDForUser(
	id string,
	userID string,
) (*models.FileUpload, error) {
	if id == "" || userID == "" {
		return nil, ErrInvalidFileUpload
	}

	return s.fileUploadRepo.FindByIDAndUser(id, userID)
}

func (s *FileUploadService) ListForUser(
	userID string,
	page int,
	pageSize int,
) ([]models.FileUpload, int64, int, int, error) {
	if userID == "" {
		return nil, 0, 0, 0, ErrInvalidFileUpload
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	files, total, err := s.fileUploadRepo.ListForUser(userID, page, pageSize)
	if err != nil {
		return nil, 0, page, pageSize, err
	}
	return files, total, page, pageSize, nil
}

func (s *FileUploadService) Reconcile(storageDirectory string) (*FileReconciliation, error) {
	files, err := s.fileUploadRepo.ListAllUnscoped()
	if err != nil {
		return nil, err
	}

	knownNames := make(map[string]bool, len(files))
	result := &FileReconciliation{
		MissingFiles:  make([]FileReconciliationItem, 0),
		OrphanedFiles: make([]string, 0),
	}
	for _, file := range files {
		knownNames[file.StoredName] = true
		if _, statErr := os.Stat(file.Path); errors.Is(statErr, os.ErrNotExist) {
			result.MissingFiles = append(result.MissingFiles, FileReconciliationItem{
				ID:           file.ID.String(),
				OriginalName: file.OriginalName,
			})
		} else if statErr != nil {
			return nil, fmt.Errorf("unable to inspect file metadata: %w", statErr)
		}
	}

	entries, err := os.ReadDir(storageDirectory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return nil, fmt.Errorf("unable to inspect upload storage: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || knownNames[filepath.Base(entry.Name())] {
			continue
		}
		result.OrphanedFiles = append(result.OrphanedFiles, entry.Name())
	}

	return result, nil
}

func (s *FileUploadService) FindByUserAndIdempotencyKey(
	userID uuid.UUID,
	idempotencyKey string,
) (*models.FileUpload, error) {
	if userID == uuid.Nil || idempotencyKey == "" {
		return nil, ErrInvalidFileUpload
	}

	return s.fileUploadRepo.FindByUserAndIdempotencyKey(userID, idempotencyKey)
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
		if errors.Is(err, repository.ErrFileUploadNotFound) {
			if _, unscopedErr := s.fileUploadRepo.FindByIDAndUserUnscoped(id, userID); unscopedErr == nil {
				return ErrFileUploadAlreadyDeleted
			}
		}
		return err
	}

	if err := s.fileUploadRepo.DeleteForUser(id, userID); err != nil {
		return fmt.Errorf("unable to delete file metadata: %w", err)
	}

	return removeFileAfterSoftDelete(
		fileUpload.Path,
		func() error { return s.fileUploadRepo.RestoreForUser(id, userID) },
	)
}

func removeFileAfterSoftDelete(path string, restore func() error) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if restoreErr := restore(); restoreErr != nil {
		return fmt.Errorf(
			"unable to delete physical file and restore metadata: %w",
			errors.Join(err, restoreErr),
		)
	}

	return fmt.Errorf("unable to delete physical file: %w", err)
}
