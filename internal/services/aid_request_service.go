package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidAidType = errors.New(
		"invalid aid type",
	)
	ErrInvalidAidPriority = errors.New(
		"invalid aid priority",
	)
	ErrInvalidAidRequestDate = errors.New(
		"invalid aid request date",
	)
	ErrInvalidNeededByDate = errors.New(
		"invalid needed-by date",
	)
	ErrInvalidAidStatus = errors.New(
		"invalid aid request status",
	)
	ErrInvalidAidDateRange = errors.New(
		"invalid aid request date range",
	)
	ErrInvalidAidRequestID = errors.New(
		"invalid aid request id",
	)
	ErrAidRequestCannotBeEdited = errors.New(
		"only pending or under-review aid requests can be edited",
	)
	ErrInvalidAidStatusTransition = errors.New(
		"invalid aid request status transition",
	)
	ErrApprovedAmountRequired = errors.New(
		"approved amount must be greater than zero",
	)
	ErrApprovedAmountTooHigh = errors.New(
		"approved amount cannot exceed requested amount",
	)
	ErrAidRequestCannotBeCancelled = errors.New(
		"this aid request cannot be cancelled",
	)
	ErrAidRequestCannotBeDeleted = errors.New(
		"only rejected or cancelled aid requests can be deleted",
	)
)

type AidRequestService struct {
	aidRequestRepo      *repository.AidRequestRepository
	personRepo          *repository.PersonRepository
	notificationService *NotificationService
}

func NewAidRequestService(
	aidRequestRepo *repository.AidRequestRepository,
	personRepo *repository.PersonRepository,
	notificationService *NotificationService,
) *AidRequestService {
	return &AidRequestService{
		aidRequestRepo:      aidRequestRepo,
		personRepo:          personRepo,
		notificationService: notificationService,
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
		parsedDate, err := parseDateValue(request.RequestDate)
		if err != nil {
			return nil, ErrInvalidAidRequestDate
		}

		requestDate = parsedDate
	}

	var neededBy *time.Time

	if strings.TrimSpace(request.NeededBy) != "" {
		parsedDate, err := parseDateValue(request.NeededBy)
		if err != nil {
			return nil, ErrInvalidNeededByDate
		}

		if parsedDate.Before(requestDate) {
			return nil, ErrInvalidNeededByDate
		}

		neededBy = &parsedDate
	}

	currency := strings.ToUpper(
		strings.TrimSpace(request.Currency),
	)

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
		return nil, fmt.Errorf(
			"unable to create aid request: %w",
			err,
		)
	}

	// Create notification for the user who created the request.
	notification := &models.Notification{
		UserID:  createdByID,
		Title:   "Aid Request Created",
		Message: "Your aid request has been created successfully.",
		Type:    "aid_request",
	}

	if s.notificationService != nil {
		if err := s.notificationService.Create(notification); err != nil {
			return nil, fmt.Errorf(
				"unable to create aid request notification: %w",
				err,
			)
		}
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
	page, pageSize := normalizePagination(
		query.Page,
		query.PageSize,
	)

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

	fromDate, toDate, err := parseDateRange(
		query.FromDate,
		query.ToDate,
	)

	if err != nil {
		return nil, 0, page, pageSize, ErrInvalidAidDateRange
	}

	query.PersonID = personID
	query.Type = aidType
	query.Priority = priority
	query.Status = status

	if fromDate != nil {
		query.FromDate = fromDate.Format(
			constants.DateLayout,
		)
	}

	if toDate != nil {
		query.ToDate = toDate.Format(
			constants.DateLayout,
		)
	}

	query.Page = page
	query.PageSize = pageSize

	aidRequests, total, err := s.aidRequestRepo.List(query)
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

func (s *AidRequestService) UpdateAidRequest(
	id string,
	request models.UpdateAidRequest,
) (*models.AidRequest, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidAidRequestID
	}

	aidRequest, err := s.aidRequestRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if !canEditAidRequest(aidRequest.Status) {
		return nil, ErrAidRequestCannotBeEdited
	}

	personID, person, err := s.validateAidRequestPerson(
		request.PersonID,
	)
	if err != nil {
		return nil, err
	}

	aidType, priority, err := validateAidRequestDetails(
		request.AidType,
		request.Priority,
	)
	if err != nil {
		return nil, err
	}

	requestDate, neededBy, err := resolveAidRequestDates(
		aidRequest.RequestDate,
		request.RequestDate,
		request.NeededBy,
	)
	if err != nil {
		return nil, err
	}

	currency := normalizeCurrency(request.Currency)

	if currency == "" {
		currency = "LKR"
	}

	aidRequest.PersonID = personID
	aidRequest.AidType = aidType
	aidRequest.Priority = priority
	aidRequest.Title = strings.TrimSpace(request.Title)
	aidRequest.Description = strings.TrimSpace(request.Description)
	aidRequest.RequestedAmount = request.RequestedAmount
	aidRequest.Currency = currency
	aidRequest.RequestDate = requestDate
	aidRequest.NeededBy = neededBy

	if err := s.aidRequestRepo.Update(aidRequest); err != nil {
		return nil, err
	}

	aidRequest.Person = *person

	return aidRequest, nil
}

func canEditAidRequest(status string) bool {
	return status == models.AidStatusPending ||
		status == models.AidStatusUnderReview
}

func (s *AidRequestService) validateAidRequestPerson(
	personID string,
) (uuid.UUID, *models.Person, error) {
	personID = strings.TrimSpace(personID)

	parsedPersonID, err := uuid.Parse(personID)
	if err != nil {
		return uuid.Nil, nil, ErrInvalidPersonID
	}

	person, err := s.personRepo.FindByID(personID)
	if err != nil {
		return uuid.Nil, nil, err
	}

	if person.Status != models.PersonStatusActive {
		return uuid.Nil, nil, errors.New(
			"person account is not active",
		)
	}

	return parsedPersonID, person, nil
}

func validateAidRequestDetails(
	aidType,
	priority string,
) (string, string, error) {
	aidType = strings.TrimSpace(aidType)

	if !isValidAidType(aidType) {
		return "", "", ErrInvalidAidType
	}

	priority = strings.TrimSpace(priority)

	if !isValidAidPriority(priority) {
		return "", "", ErrInvalidAidPriority
	}

	return aidType, priority, nil
}

func resolveAidRequestDates(
	requestDate time.Time,
	requestDateValue string,
	neededByValue string,
) (time.Time, *time.Time, error) {
	resolvedRequestDate := requestDate

	if strings.TrimSpace(requestDateValue) != "" {
		parsedDate, err := parseDateValue(requestDateValue)
		if err != nil {
			return time.Time{}, nil, ErrInvalidAidRequestDate
		}

		resolvedRequestDate = parsedDate
	}

	var neededBy *time.Time

	if strings.TrimSpace(neededByValue) != "" {
		parsedDate, err := parseDateValue(neededByValue)
		if err != nil {
			return time.Time{}, nil, ErrInvalidNeededByDate
		}

		if parsedDate.Before(resolvedRequestDate) {
			return time.Time{}, nil, ErrInvalidNeededByDate
		}

		neededBy = &parsedDate
	}

	return resolvedRequestDate, neededBy, nil
}

func isValidAidStatusTransition(
	currentStatus,
	newStatus string,
) bool {
	switch currentStatus {
	case models.AidStatusPending:
		return newStatus == models.AidStatusUnderReview ||
			newStatus == models.AidStatusCancelled

	case models.AidStatusUnderReview:
		return newStatus == models.AidStatusApproved ||
			newStatus == models.AidStatusRejected ||
			newStatus == models.AidStatusCancelled

	case models.AidStatusApproved:
		return newStatus == models.AidStatusCompleted ||
			newStatus == models.AidStatusCancelled

	default:
		return false
	}
}

func (s *AidRequestService) ReviewAidRequest(
	id string,
	request models.ReviewAidRequest,
	reviewerID uuid.UUID,
) (*models.AidRequest, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidAidRequestID
	}

	aidRequest, err := s.aidRequestRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	newStatus := strings.TrimSpace(request.Status)

	if !isValidAidStatus(newStatus) {
		return nil, ErrInvalidAidStatus
	}

	if !isValidAidStatusTransition(
		aidRequest.Status,
		newStatus,
	) {
		return nil, ErrInvalidAidStatusTransition
	}

	switch newStatus {
	case models.AidStatusApproved:
		if request.ApprovedAmount <= 0 {
			return nil, ErrApprovedAmountRequired
		}

		if request.ApprovedAmount >
			aidRequest.RequestedAmount {
			return nil, ErrApprovedAmountTooHigh
		}

		aidRequest.ApprovedAmount =
			request.ApprovedAmount

	case models.AidStatusRejected,
		models.AidStatusCancelled:
		aidRequest.ApprovedAmount = 0
	}

	now := time.Now().UTC()

	aidRequest.Status = newStatus

	aidRequest.ReviewNotes = strings.TrimSpace(
		request.ReviewNotes,
	)

	aidRequest.ReviewedByID = &reviewerID
	aidRequest.ReviewedAt = &now

	if err := s.aidRequestRepo.UpdateReview(
		aidRequest,
	); err != nil {
		return nil, err
	}

	// Notify the user who created the aid request.
	title := "Aid Request Updated"
	message := "Your aid request status has been updated."

	switch newStatus {
	case models.AidStatusUnderReview:
		title = "Aid Request Under Review"
		message = "Your aid request is now under review."

	case models.AidStatusApproved:
		title = "Aid Request Approved"
		message = "Your aid request has been approved."

	case models.AidStatusRejected:
		title = "Aid Request Rejected"
		message = "Your aid request has been rejected."

	case models.AidStatusCancelled:
		title = "Aid Request Cancelled"
		message = "Your aid request has been cancelled."

	case models.AidStatusCompleted:
		title = "Aid Request Completed"
		message = "Your aid request has been completed."
	}

	notification := &models.Notification{
		UserID:  aidRequest.CreatedByID,
		Title:   title,
		Message: message,
		Type:    "aid_request",
	}

	if s.notificationService != nil {
		if err := s.notificationService.Create(
			notification,
		); err != nil {
			return nil, fmt.Errorf(
				"unable to create aid request notification: %w",
				err,
			)
		}
	}

	return aidRequest, nil
}

func (s *AidRequestService) CancelAidRequest(
	id string,
	reason string,
	reviewerID uuid.UUID,
) (*models.AidRequest, error) {
	id = strings.TrimSpace(id)
	reason = strings.TrimSpace(reason)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidAidRequestID
	}

	aidRequest, err := s.aidRequestRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	switch aidRequest.Status {
	case models.AidStatusPending,
		models.AidStatusUnderReview,
		models.AidStatusApproved:
		// Cancellation allowed.

	default:
		return nil, ErrAidRequestCannotBeCancelled
	}

	now := time.Now().UTC()

	aidRequest.Status = models.AidStatusCancelled
	aidRequest.ReviewNotes = reason
	aidRequest.ReviewedByID = &reviewerID
	aidRequest.ReviewedAt = &now

	if err := s.aidRequestRepo.UpdateReview(
		aidRequest,
	); err != nil {
		return nil, err
	}

	notification := &models.Notification{
		UserID:  aidRequest.CreatedByID,
		Title:   "Aid Request Cancelled",
		Message: "Your aid request has been cancelled.",
		Type:    "aid_request",
	}

	if s.notificationService != nil {
		if err := s.notificationService.Create(
			notification,
		); err != nil {
			return nil, fmt.Errorf(
				"unable to create cancellation notification: %w",
				err,
			)
		}
	}

	return aidRequest, nil
}

func (s *AidRequestService) DeleteAidRequest(
	id string,
) error {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidAidRequestID
	}

	aidRequest, err := s.aidRequestRepo.FindByID(id)
	if err != nil {
		return err
	}

	if aidRequest.Status != models.AidStatusRejected &&
		aidRequest.Status != models.AidStatusCancelled {
		return ErrAidRequestCannotBeDeleted
	}

	if err := s.aidRequestRepo.SoftDelete(aidRequest); err != nil {
		return err
	}

	return nil
}
