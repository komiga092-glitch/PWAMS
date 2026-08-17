package services

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidCareProvidedID     = errors.New("invalid care provided ID")
	ErrInvalidCareProvidedStatus = errors.New("invalid care provided status")
	ErrCareAlreadyCompleted      = errors.New("care provided record is already completed")
)

type CareProvidedService struct {
	careProvidedRepo *repository.CareProvidedRepository
}

func NewCareProvidedService(
	careProvidedRepo *repository.CareProvidedRepository,
) *CareProvidedService {
	return &CareProvidedService{
		careProvidedRepo: careProvidedRepo,
	}
}

func isValidCareProvidedStatus(status string) bool {
	switch status {
	case models.CareProvidedStatusPending,
		models.CareProvidedStatusCompleted,
		models.CareProvidedStatusCancelled:
		return true
	default:
		return false
	}
}

func (s *CareProvidedService) CreateCareProvided(
	request models.CreateCareProvidedRequest,
	createdByID string,
) (*models.CareProvided, error) {
	aidRequestID, err := uuid.Parse(
		strings.TrimSpace(request.AidRequestID),
	)
	if err != nil {
		return nil, errors.New("invalid aid request ID")
	}

	personID, err := uuid.Parse(
		strings.TrimSpace(request.PersonID),
	)
	if err != nil {
		return nil, errors.New("invalid person ID")
	}

	userID, err := uuid.Parse(strings.TrimSpace(createdByID))
	if err != nil {
		return nil, errors.New("invalid created by user ID")
	}

	providedAt, err := time.Parse(
		time.RFC3339,
		strings.TrimSpace(request.ProvidedAt),
	)
	if err != nil {
		return nil, errors.New("invalid provided date")
	}

	careProvided := &models.CareProvided{
		ID:           uuid.New(),
		AidRequestID: aidRequestID,
		PersonID:     personID,
		Amount:       request.Amount,
		Description:  strings.TrimSpace(request.Description),
		Status:       models.CareProvidedStatusPending,
		ProvidedAt:   providedAt,
		CreatedByID:  userID,
	}

	if err := s.careProvidedRepo.Create(careProvided); err != nil {
		return nil, err
	}

	return s.careProvidedRepo.FindByID(careProvided.ID)
}

func (s *CareProvidedService) GetCareProvidedByID(
	id string,
) (*models.CareProvided, error) {
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, ErrInvalidCareProvidedID
	}

	return s.careProvidedRepo.FindByID(parsedID)
}

func (s *CareProvidedService) ListCareProvided(
	page int,
	pageSize int,
) ([]models.CareProvided, int64, int, int, error) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 20
	}

	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	records, total, err := s.careProvidedRepo.List(
		offset,
		pageSize,
	)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	totalPages := 0

	if total > 0 {
		totalPages = int(
			(total + int64(pageSize) - 1) /
				int64(pageSize),
		)
	}

	return records, total, page, totalPages, nil
}

func (s *CareProvidedService) UpdateCareProvided(
	id string,
	request models.UpdateCareProvidedRequest,
) (*models.CareProvided, error) {
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, ErrInvalidCareProvidedID
	}

	careProvided, err := s.careProvidedRepo.FindByID(parsedID)
	if err != nil {
		return nil, err
	}

	if careProvided.Status == models.CareProvidedStatusCompleted {
		return nil, ErrCareAlreadyCompleted
	}

	providedAt, err := time.Parse(
		time.RFC3339,
		strings.TrimSpace(request.ProvidedAt),
	)
	if err != nil {
		return nil, errors.New("invalid provided date")
	}

	careProvided.Amount = request.Amount
	careProvided.Description = strings.TrimSpace(request.Description)
	careProvided.ProvidedAt = providedAt

	if err := s.careProvidedRepo.Update(careProvided); err != nil {
		return nil, err
	}

	return s.careProvidedRepo.FindByID(parsedID)
}

func (s *CareProvidedService) UpdateCareProvidedStatus(
	id string,
	status string,
) error {
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return ErrInvalidCareProvidedID
	}

	status = strings.TrimSpace(status)

	if !isValidCareProvidedStatus(status) {
		return ErrInvalidCareProvidedStatus
	}

	careProvided, err := s.careProvidedRepo.FindByID(parsedID)
	if err != nil {
		return err
	}

	if careProvided.Status == models.CareProvidedStatusCompleted &&
		status != models.CareProvidedStatusCompleted {
		return ErrCareAlreadyCompleted
	}

	return s.careProvidedRepo.UpdateStatus(
		parsedID,
		status,
	)
}

func (s *CareProvidedService) DeleteCareProvided(
	id string,
) error {
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return ErrInvalidCareProvidedID
	}

	careProvided, err := s.careProvidedRepo.FindByID(parsedID)
	if err != nil {
		return err
	}

	if careProvided.Status == models.CareProvidedStatusCompleted {
		return ErrCareAlreadyCompleted
	}

	return s.careProvidedRepo.Delete(parsedID)
}
