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
	ErrInvalidAidStatus      = errors.New(
		"invalid aid request status",
	)

	ErrInvalidAidDateRange = errors.New(
		"invalid aid request date range",
	)
	ErrInvalidAidRequestID = errors.New("invalid aid request id")
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
func isValidAidStatus(value string) bool {
	switch value {
	case models.AidStatusPending,
		models.AidStatusUnderReview,
		models.AidStatusApproved,
		models.AidStatusRejected,
		models.AidStatusCompleted,
		models.AidStatusCancelled:
		return true

	default:
		return false
	}
}
func (s *AidRequestService) ListAidRequests(
	query models.AidRequestListQuery,
) ([]models.AidRequest, int64, int, int, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}

	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	if pageSize > 100 {
		pageSize = 100
	}

	personID := strings.TrimSpace(query.PersonID)

	if personID != "" {
		if _, err := uuid.Parse(personID); err != nil {
			return nil, 0, page, pageSize, ErrInvalidPersonID
		}
	}

	aidType := strings.TrimSpace(query.Type)

	if aidType != "" && !isValidAidType(aidType) {
		return nil, 0, page, pageSize, ErrInvalidAidType
	}

	priority := strings.TrimSpace(query.Priority)

	if priority != "" && !isValidAidPriority(priority) {
		return nil, 0, page, pageSize, ErrInvalidAidPriority
	}

	status := strings.TrimSpace(query.Status)

	if status != "" && !isValidAidStatus(status) {
		return nil, 0, page, pageSize, ErrInvalidAidStatus
	}

	var fromDate *time.Time
	var toDate *time.Time

	if strings.TrimSpace(query.FromDate) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			query.FromDate,
		)
		if err != nil {
			return nil, 0, page, pageSize,
				ErrInvalidAidDateRange
		}

		fromDate = &parsedDate
	}

	if strings.TrimSpace(query.ToDate) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			query.ToDate,
		)
		if err != nil {
			return nil, 0, page, pageSize,
				ErrInvalidAidDateRange
		}

		toDate = &parsedDate
	}

	if fromDate != nil &&
		toDate != nil &&
		fromDate.After(*toDate) {
		return nil, 0, page, pageSize,
			ErrInvalidAidDateRange
	}

	aidRequests, total, err := s.aidRequestRepo.List(
		query.Search,
		personID,
		aidType,
		priority,
		status,
		fromDate,
		toDate,
		page,
		pageSize,
	)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return aidRequests, total, page, pageSize, nil
}
func (s *AidRequestService) GetAidRequestByID(
	id string,
) (*models.AidRequest, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidAidRequestID
	}

	aidRequest, err := s.aidRequestRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	return aidRequest, nil
}
