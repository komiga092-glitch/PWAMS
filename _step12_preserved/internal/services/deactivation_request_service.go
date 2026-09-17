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
	ErrDeactivationTargetNotFound  = errors.New("target user not found")
	ErrDeactivationSelfRequest     = errors.New("you cannot request the deactivation of your own account")
	ErrDeactivationTargetRole      = errors.New("deactivation requests can only target Super Admin accounts")
	ErrDeactivationDuplicate       = errors.New("a pending deactivation request already exists for this user")
	ErrDeactivationRequestNotFound = errors.New("deactivation request not found")
	ErrDeactivationAlreadyResolved = errors.New("deactivation request has already been resolved")
	ErrDeactivationNotResponder    = errors.New("only the target user can respond to this deactivation request")
)

// DeactivationRequestService implements the supervised Super Admin
// de-activation workflow: a Super Admin may not directly deactivate another
// Super Admin; instead a request is created, the target is notified, and the
// target either accepts (account is deactivated) or rejects it.
type DeactivationRequestService struct {
	deactivationRepo *repository.DeactivationRequestRepository
	userRepo         *repository.UserRepository
	auditLogService  *AuditLogService
}

func NewDeactivationRequestService(
	deactivationRepo *repository.DeactivationRequestRepository,
	userRepo *repository.UserRepository,
	auditLogService *AuditLogService,
) *DeactivationRequestService {
	return &DeactivationRequestService{
		deactivationRepo: deactivationRepo,
		userRepo:         userRepo,
		auditLogService:  auditLogService,
	}
}

// Create validates the target and records a new pending request.
func (s *DeactivationRequestService) Create(
	requesterID uuid.UUID,
	request models.CreateDeactivationRequest,
) (*models.DeactivationRequest, error) {
	if strings.TrimSpace(request.TargetUserID) == "" {
		return nil, ErrDeactivationTargetNotFound
	}

	targetID, err := uuid.Parse(strings.TrimSpace(request.TargetUserID))
	if err != nil {
		return nil, ErrDeactivationTargetNotFound
	}

	if targetID == requesterID {
		return nil, ErrDeactivationSelfRequest
	}

	target, err := s.userRepo.FindByID(targetID.String())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrDeactivationTargetNotFound
		}
		return nil, err
	}

	if !strings.EqualFold(target.Role.Name, models.RoleSuperAdmin) {
		return nil, ErrDeactivationTargetRole
	}

	// Only one pending request per target at a time.
	if _, err := s.deactivationRepo.FindPendingForTarget(targetID); err == nil {
		return nil, ErrDeactivationDuplicate
	} else if !errors.Is(err, ErrDeactivationRequestNotFound) {
		return nil, err
	}

	record := &models.DeactivationRequest{
		TargetUserID: targetID,
		RequesterID:  requesterID,
		Reason:       strings.TrimSpace(request.Reason),
		Status:       models.DeactivationStatusPending,
		RequestedAt:  time.Now().UTC(),
	}

	if err := s.deactivationRepo.Create(record); err != nil {
		return nil, err
	}

	s.audit(requesterID, "REQUEST_DEACTIVATION", record.ID, fmt.Sprintf("Deactivation requested for Super Admin %s", targetID))

	return record, nil
}

// ListForUser returns requests where the user is requester or target.
func (s *DeactivationRequestService) ListForUser(
	userID uuid.UUID,
	page, pageSize int,
) ([]models.DeactivationRequest, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.deactivationRepo.ListForUser(userID, page, pageSize)
}

// Respond resolves a pending request. Only the target user may respond.
// Acceptance deactivates the target account.
func (s *DeactivationRequestService) Respond(
	requestID uuid.UUID,
	responderID uuid.UUID,
	accept bool,
	response string,
) (*models.DeactivationRequest, error) {
	record, err := s.deactivationRepo.FindByID(requestID)
	if err != nil {
		if errors.Is(err, ErrDeactivationRequestNotFound) {
			return nil, ErrDeactivationRequestNotFound
		}
		return nil, err
	}

	if record.Status != models.DeactivationStatusPending {
		return nil, ErrDeactivationAlreadyResolved
	}

	if record.TargetUserID != responderID {
		return nil, ErrDeactivationNotResponder
	}

	status := models.DeactivationStatusRejected
	action := "REJECT_DEACTIVATION"
	if accept {
		status = models.DeactivationStatusAccepted
		action = "ACCEPT_DEACTIVATION"

		if err := s.userRepo.UpdateStatus(record.TargetUserID.String(), models.UserStatusDisabled); err != nil {
			return nil, fmt.Errorf("failed to deactivate target account: %w", err)
		}
	}

	if err := s.deactivationRepo.Respond(record.ID, status, strings.TrimSpace(response), responderID); err != nil {
		return nil, err
	}

	record.Status = status
	now := time.Now().UTC()
	record.RespondedAt = &now
	record.Response = strings.TrimSpace(response)
	record.RespondedByID = &responderID

	details := fmt.Sprintf("%s deactivation request for %s", status, record.TargetUserID)
	if record.Response != "" {
		details += ": " + record.Response
	}
	s.audit(responderID, action, record.ID, details)

	return record, nil
}

// ExpireStale marks pending requests older than the given age as expired.
func (s *DeactivationRequestService) ExpireStale(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	count, err := s.deactivationRepo.ExpireStale(cutoff)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		_ = s.auditLogService.Create("", "EXPIRE_DEACTIVATION", "deactivation_requests", "", fmt.Sprintf("%d stale requests expired", count))
	}
	return count, nil
}

func (s *DeactivationRequestService) audit(userID uuid.UUID, action string, entityID uuid.UUID, details string) {
	if s.auditLogService == nil {
		return
	}
	_ = s.auditLogService.Create(userID.String(), action, "deactivation_requests", entityID.String(), details)
}
