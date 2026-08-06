package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidAidType        = errors.New("invalid aid type")
	ErrInvalidAidPriority    = errors.New("invalid aid priority")
	ErrInvalidAidRequestDate = errors.New("invalid aid request date")
	ErrInvalidNeededByDate   = errors.New("invalid needed-by date")
)

type AidRequestService struct {
	aidRequestRepo *repository.AidRequestRepository
	personRepo     *repository.PersonRepository
}

func NewAidRequestService(
	aidRequestRepo *repository.AidRequestRepository,
	personRepo *repository.PersonRepository,
) *AidRequestService {
	return &AidRequestService{
		aidRequestRepo: aidRequestRepo,
		personRepo:     personRepo,
	}
}

func (s *AidRequestService) CreateAidRequest(
	request models.CreateAidRequest,
	createdByID uuid.UUID,
) (*models.AidRequest, error) {
	personID := strings.TrimSpace(request.PersonID)

	parsedPersonID, err := uuid.Parse(personID)
	if err != nil {
		return nil, ErrInvalidPersonID
	}

	person, err := s.personRepo.FindByID(personID)
	if err != nil {
		return nil, err
	}

	if person.Status != models.PersonStatusActive {
		return nil, errors.New("person account is not active")
	}

	aidType := strings.TrimSpace(request.AidType)
	if !isValidAidType(aidType) {
		return nil, ErrInvalidAidType
	}

	priority := strings.TrimSpace(request.Priority)
	if !isValidAidPriority(priority) {
		return nil, ErrInvalidAidPriority
	}

	requestDate := time.Now().UTC()

	if strings.TrimSpace(request.RequestDate) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			request.RequestDate,
		)
		if err != nil {
			return nil, ErrInvalidAidRequestDate
		}

		requestDate = parsedDate
	}

	var neededBy *time.Time

	if strings.TrimSpace(request.NeededBy) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			request.NeededBy,
		)
		if err != nil {
			return nil, ErrInvalidNeededByDate
		}

		if parsedDate.Before(requestDate) {
			return nil, ErrInvalidNeededByDate
		}

		neededBy = &parsedDate
	}

	currency := strings.ToUpper(strings.TrimSpace(request.Currency))
	if currency == "" {
		currency = "LKR"
	}

	aidRequest := &models.AidRequest{
		PersonID:        parsedPersonID,
		AidType:         aidType,
		Priority:        priority,
		Title:           strings.TrimSpace(request.Title),
		Description:     strings.TrimSpace(request.Description),
		RequestedAmount: request.RequestedAmount,
		ApprovedAmount:  0,
		Currency:        currency,
		RequestDate:     requestDate,
		NeededBy:        neededBy,
		Status:          models.AidStatusPending,
		CreatedByID:     createdByID,
	}

	if err := s.aidRequestRepo.Create(aidRequest); err != nil {
		return nil, fmt.Errorf("unable to create aid request: %w", err)
	}

	return aidRequest, nil
}

func isValidAidType(value string) bool {
	switch value {
	case models.AidTypeMedical,
		models.AidTypeEducation,
		models.AidTypeFood,
		models.AidTypeHousing,
		models.AidTypeClothing,
		models.AidTypeEmergency,
		models.AidTypeOther:
		return true

	default:
		return false
	}
}

func isValidAidPriority(value string) bool {
	switch value {
	case models.AidPriorityLow,
		models.AidPriorityMedium,
		models.AidPriorityHigh,
		models.AidPriorityCritical:
		return true

	default:
		return false
	}
}
