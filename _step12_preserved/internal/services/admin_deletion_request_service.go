package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrDeletionTargetNotFound  = errors.New("target user not found")
	ErrDeletionSelfRequest     = errors.New("you cannot request the deletion of your own account")
	ErrDeletionTargetRole      = errors.New("deletion requests can only target Admin accounts")
	ErrDeletionDuplicate       = errors.New("a pending deletion request already exists for this Admin account")
	ErrDeletionRequestNotFound = errors.New("admin deletion request not found")
	ErrDeletionAlreadyResolved = errors.New("admin deletion request has already been resolved")
	ErrDeletionSelfApproval    = errors.New("the actor who raised a deletion request cannot approve or reject it")
	ErrDeletionTargetApproval  = errors.New("the target of a deletion request cannot approve or reject it")
	ErrDeletionCancelForbidden = errors.New("only the requester may cancel a deletion request")
	ErrDeletionHierarchy       = errors.New("this action is not permitted by the role hierarchy")
)

// canRequestAdminDeletion validates that an actor with requesterRole may
// raise (or resolve) a deletion request against an account with targetRole.
// Only Admin accounts are deletable through the workflow, and only actors
// that may manage Admin accounts (Super Admin or Admin — multi-Admin) may
// request the deletion. In particular an Admin can never target a Super
// Admin account and an operational role can never target anything.
func canRequestAdminDeletion(requesterRole, targetRole string) error {
	if targetRole != models.RoleAdmin {
		return ErrDeletionTargetRole
	}
	if !CanManageAccountRole(requesterRole, targetRole) {
		return ErrDeletionHierarchy
	}
	return nil
}

// canResolveAdminDeletion enforces the two independent subject guards of the
// deletion workflow: the actor approving or rejecting a request must not be
// the actor who raised it (four-eyes rule, so a single Admin can never
// unilaterally remove another Admin account), and must not be the target of
// the request either — the subject of a deletion decision never decides
// their own removal.
func canResolveAdminDeletion(requesterID, resolverID, targetID uuid.UUID) error {
	if requesterID == resolverID {
		return ErrDeletionSelfApproval
	}
	if targetID == resolverID {
		return ErrDeletionTargetApproval
	}
	return nil
}

// AdminDeletionRequestService implements the supervised Admin account
// deletion workflow (admin.delete.request / admin.delete.approve /
// admin.delete.reject). A deletion request may be raised by any actor that
// manages Admin accounts; it must then be approved or rejected by a
// DIFFERENT actor. Approval executes the deletion through UserService
// (session revocation + soft delete + hierarchy enforcement), so the
// workflow can never delete a Super Admin account or the acting account
// itself.
type AdminDeletionRequestService struct {
	deletionRepo    *repository.AdminDeletionRequestRepository
	userRepo        *repository.UserRepository
	userService     *UserService
	auditLogService *AuditLogService
}

func NewAdminDeletionRequestService(
	deletionRepo *repository.AdminDeletionRequestRepository,
	userRepo *repository.UserRepository,
	userService *UserService,
	auditLogService *AuditLogService,
) *AdminDeletionRequestService {
	return &AdminDeletionRequestService{
		deletionRepo:    deletionRepo,
		userRepo:        userRepo,
		userService:     userService,
		auditLogService: auditLogService,
	}
}

// Create validates the target and records a new pending deletion request.
func (s *AdminDeletionRequestService) Create(
	requester *models.User,
	request models.CreateAdminDeletionRequest,
) (*models.AdminDeletionRequest, error) {
	if requester == nil {
		return nil, ErrDeletionHierarchy
	}
	if strings.TrimSpace(request.TargetUserID) == "" {
		return nil, ErrDeletionTargetNotFound
	}

	targetID, err := uuid.Parse(strings.TrimSpace(request.TargetUserID))
	if err != nil {
		return nil, ErrDeletionTargetNotFound
	}

	if targetID == requester.ID {
		return nil, ErrDeletionSelfRequest
	}

	target, err := s.userRepo.FindByID(targetID.String())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrDeletionTargetNotFound
		}
		return nil, err
	}

	if err := canRequestAdminDeletion(requester.Role.Name, target.Role.Name); err != nil {
		return nil, err
	}

	// Only one pending request per target at a time.
	if _, err := s.deletionRepo.FindPendingForTarget(targetID); err == nil {
		return nil, ErrDeletionDuplicate
	} else if !errors.Is(err, repository.ErrAdminDeletionRequestNotFound) {
		return nil, err
	}

	record := &models.AdminDeletionRequest{
		TargetUserID: targetID,
		RequesterID:  requester.ID,
		Reason:       strings.TrimSpace(request.Reason),
		Status:       models.DeletionStatusPending,
		RequestedAt:  time.Now().UTC(),
	}

	if err := s.deletionRepo.Create(record); err != nil {
		// A concurrent request for the same target may have won the insert
		// race; the partial unique index on pending requests rejects it at
		// the database level. Surface the friendly duplicate error then.
		if _, dupErr := s.deletionRepo.FindPendingForTarget(targetID); dupErr == nil {
			return nil, ErrDeletionDuplicate
		}
		return nil, err
	}

	s.audit(requester.ID, "REQUEST_ADMIN_DELETION", record.ID,
		fmt.Sprintf("Deletion requested for Admin account %s", targetID))

	return record, nil
}

// ListPending returns pending deletion requests, newest first.
func (s *AdminDeletionRequestService) ListPending(
	page, pageSize int,
) ([]models.AdminDeletionRequest, int64, error) {
	page, pageSize = normaliseDeletionListPaging(page, pageSize)
	return s.deletionRepo.ListPending(page, pageSize)
}

// ListForUser returns requests where the user is requester or target.
func (s *AdminDeletionRequestService) ListForUser(
	userID uuid.UUID,
	page, pageSize int,
) ([]models.AdminDeletionRequest, int64, error) {
	page, pageSize = normaliseDeletionListPaging(page, pageSize)
	return s.deletionRepo.ListForUser(userID, page, pageSize)
}

func normaliseDeletionListPaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// Approve resolves a pending request. The approver must differ from the
// requester (four-eyes) and from the target, and must still be permitted to
// manage the target account. The whole resolution is atomic: the request row
// is locked FOR UPDATE, the guards are re-validated under that lock, the
// account is deactivated (session revocation + soft delete through
// UserService, which also re-enforces the role hierarchy so a Super Admin
// target is impossible and the approver can never delete themselves), and
// the request status flip lands in the same transaction. A concurrent
// approval therefore either observes the row locked and blocks, or sees the
// status no longer pending and fails without touching the account.
func (s *AdminDeletionRequestService) Approve(
	approver *models.User,
	requestID uuid.UUID,
	response string,
) (*models.AdminDeletionRequest, error) {
	if approver == nil {
		return nil, ErrDeletionHierarchy
	}

	var record *models.AdminDeletionRequest
	err := s.deletionRepo.Transaction(func(tx *gorm.DB) error {
		current, err := s.deletionRepo.FindByIDForUpdate(tx, requestID)
		if err != nil {
			if errors.Is(err, repository.ErrAdminDeletionRequestNotFound) {
				return ErrDeletionRequestNotFound
			}
			return err
		}

		// The row lock guarantees the status cannot change between this
		// check and the status flip at the end of the transaction.
		if current.Status != models.DeletionStatusPending {
			return ErrDeletionAlreadyResolved
		}

		if err := canResolveAdminDeletion(current.RequesterID, approver.ID, current.TargetUserID); err != nil {
			return err
		}

		target, err := s.userRepo.WithTx(tx).FindByID(current.TargetUserID.String())
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return ErrDeletionTargetNotFound
			}
			return err
		}

		// Re-validate the workflow guards at resolution time: the target's
		// role may have changed after the request was raised (e.g. it was
		// promoted), and the approver's own rights may have changed.
		if err := canRequestAdminDeletion(approver.Role.Name, target.Role.Name); err != nil {
			return err
		}

		// Deactivate the account inside the same transaction: sessions are
		// revoked and the row is soft-deleted together with the status flip.
		if err := s.userService.DeleteUserInTx(
			tx,
			current.TargetUserID.String(),
			approver.ID.String(),
			approver.Role.Name,
		); err != nil {
			return err
		}

		if err := s.deletionRepo.RespondTx(
			tx,
			current.ID,
			models.DeletionStatusApproved,
			strings.TrimSpace(response),
			approver.ID,
		); err != nil {
			return err
		}

		record = current
		return nil
	})
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	record.Status = models.DeletionStatusApproved
	record.RespondedAt = &now
	record.Response = strings.TrimSpace(response)
	record.RespondedByID = &approver.ID

	details := fmt.Sprintf("Admin deletion request for %s approved by %s", record.TargetUserID, approver.ID)
	if record.Response != "" {
		details += ": " + record.Response
	}
	s.audit(approver.ID, "APPROVE_ADMIN_DELETION", record.ID, details)

	// The approval executed a supervised account deactivation (soft delete
	// + session revocation); record it against the target account as well.
	s.audit(approver.ID, "DEACTIVATE_ADMIN", record.TargetUserID, details)

	return record, nil
}

// Reject resolves a pending request without deleting the target account.
// The responder must differ from the requester (four-eyes rule, applied
// symmetrically so a request is always resolved by a second actor) and from
// the target. The status flip runs under the request row lock so a
// concurrent resolution cannot interleave.
func (s *AdminDeletionRequestService) Reject(
	responder *models.User,
	requestID uuid.UUID,
	response string,
) (*models.AdminDeletionRequest, error) {
	if responder == nil {
		return nil, ErrDeletionHierarchy
	}

	var record *models.AdminDeletionRequest
	err := s.deletionRepo.Transaction(func(tx *gorm.DB) error {
		current, err := s.deletionRepo.FindByIDForUpdate(tx, requestID)
		if err != nil {
			if errors.Is(err, repository.ErrAdminDeletionRequestNotFound) {
				return ErrDeletionRequestNotFound
			}
			return err
		}

		if current.Status != models.DeletionStatusPending {
			return ErrDeletionAlreadyResolved
		}

		if err := canResolveAdminDeletion(current.RequesterID, responder.ID, current.TargetUserID); err != nil {
			return err
		}

		if err := s.deletionRepo.RespondTx(
			tx,
			current.ID,
			models.DeletionStatusRejected,
			strings.TrimSpace(response),
			responder.ID,
		); err != nil {
			return err
		}

		record = current
		return nil
	})
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	record.Status = models.DeletionStatusRejected
	record.RespondedAt = &now
	record.Response = strings.TrimSpace(response)
	record.RespondedByID = &responder.ID

	details := fmt.Sprintf("Admin deletion request for %s rejected by %s", record.TargetUserID, responder.ID)
	if record.Response != "" {
		details += ": " + record.Response
	}
	s.audit(responder.ID, "REJECT_ADMIN_DELETION", record.ID, details)

	return record, nil
}

// Cancel lets the requester withdraw their own still-pending request. The
// target account is never touched: cancellation only resolves the request to
// cancelled, which also releases the target for a future request.
func (s *AdminDeletionRequestService) Cancel(
	requester *models.User,
	requestID uuid.UUID,
) (*models.AdminDeletionRequest, error) {
	if requester == nil {
		return nil, ErrDeletionHierarchy
	}

	var record *models.AdminDeletionRequest
	err := s.deletionRepo.Transaction(func(tx *gorm.DB) error {
		current, err := s.deletionRepo.FindByIDForUpdate(tx, requestID)
		if err != nil {
			if errors.Is(err, repository.ErrAdminDeletionRequestNotFound) {
				return ErrDeletionRequestNotFound
			}
			return err
		}

		if current.RequesterID != requester.ID {
			return ErrDeletionCancelForbidden
		}

		if current.Status != models.DeletionStatusPending {
			return ErrDeletionAlreadyResolved
		}

		if err := s.deletionRepo.RespondTx(
			tx,
			current.ID,
			models.DeletionStatusCancelled,
			"",
			requester.ID,
		); err != nil {
			return err
		}

		record = current
		return nil
	})
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	record.Status = models.DeletionStatusCancelled
	record.RespondedAt = &now
	record.RespondedByID = &requester.ID

	details := fmt.Sprintf("Admin deletion request for %s cancelled by its requester %s", record.TargetUserID, requester.ID)
	s.audit(requester.ID, "CANCEL_ADMIN_DELETION", record.ID, details)

	return record, nil
}

// ExpireStale marks pending requests older than the given age as expired.
func (s *AdminDeletionRequestService) ExpireStale(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	count, err := s.deletionRepo.ExpireStale(cutoff)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		_ = s.auditLogService.Create("", "EXPIRE_ADMIN_DELETION", "admin_deletion_requests", "", fmt.Sprintf("%d stale requests expired", count))
	}
	return count, nil
}

func (s *AdminDeletionRequestService) audit(userID uuid.UUID, action string, entityID uuid.UUID, details string) {
	if s.auditLogService == nil {
		return
	}
	_ = s.auditLogService.Create(userID.String(), action, "admin_deletion_requests", entityID.String(), details)
}
