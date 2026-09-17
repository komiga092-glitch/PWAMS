package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// Admin deletion workflow errors. HTTP mappings live in
// internal/handlers/admin_deletion_handler.go.
var (
	// ErrLastActiveAdmin guards the invariant that the system must always
	// keep at least one active Admin account.
	ErrLastActiveAdmin = errors.New(
		"at least one active Admin account must remain",
	)

	// ErrLastActiveSuperAdmin guards the equivalent invariant for
	// Super Admin accounts.
	ErrLastActiveSuperAdmin = errors.New(
		"at least one active Super Admin account must remain",
	)

	// ErrAdminDeletionNotAuthorized is returned when the acting user holds
	// a role that may not raise or decide Admin deletion requests.
	ErrAdminDeletionNotAuthorized = errors.New(
		"you are not authorized to request or decide Admin deletions",
	)

	// ErrAdminDeletionApprovalRequired is returned when the four-eyes rule
	// blocks a decision: whoever raised a deletion request can never
	// approve or reject it themselves.
	ErrAdminDeletionApprovalRequired = errors.New(
		"admin deletion requires approval by another authorized Admin",
	)

	// ErrAdminDeletionRequestInvalid is returned when a deletion request
	// cannot be acted on (already resolved, wrong target role, duplicate
	// pending request, ...).
	ErrAdminDeletionRequestInvalid = errors.New(
		"admin deletion request is invalid or not actionable",
	)
)

// isAuthorizedAdminDecider reports whether the given role may raise or
// decide Admin deletion requests. Only Super Admin and Admin (the roles
// that manage Admin accounts) qualify; operational roles never pass.
func isAuthorizedAdminDecider(role string) bool {
	return role == models.RoleSuperAdmin || role == models.RoleAdmin
}

// RequestAdminDeletion records a Pending deletion request against an
// Admin account. Only authorized actors (Super Admin / Admin) may raise
// it, only Admin accounts may be targeted, and the request must then be
// decided by a DIFFERENT authorized actor (four-eyes rule, enforced at
// decision time).
func (s *UserService) RequestAdminDeletion(
	requestedByUserID, targetUserID string,
) (*models.AdminDeletionRequest, error) {
	requestedByUserID = strings.TrimSpace(requestedByUserID)
	targetUserID = strings.TrimSpace(targetUserID)

	requesterID, err := uuid.Parse(requestedByUserID)
	if err != nil {
		return nil, ErrInvalidUserID
	}

	targetID, err := uuid.Parse(targetUserID)
	if err != nil {
		return nil, ErrInvalidUserID
	}

	if s.adminDeletionRepo == nil {
		return nil, fmt.Errorf("admin deletion workflow is not configured")
	}

	if targetID == requesterID {
		return nil, ErrCannotDeleteSelf
	}

	requester, err := s.userRepo.FindByID(requestedByUserID)
	if err != nil {
		return nil, err
	}
	if !isAuthorizedAdminDecider(requester.Role.Name) {
		return nil, ErrAdminDeletionNotAuthorized
	}

	target, err := s.userRepo.FindByID(targetUserID)
	if err != nil {
		return nil, err
	}
	// Only Admin accounts are deletable through the approval workflow.
	if target.Role.Name != models.RoleAdmin {
		return nil, ErrAdminDeletionRequestInvalid
	}

	pending, err := s.adminDeletionRepo.FindPendingByTargetID(targetID)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		return nil, fmt.Errorf(
			"%w: a pending deletion request already exists for this Admin account",
			ErrAdminDeletionRequestInvalid,
		)
	}

	request, err := s.adminDeletionRepo.CreatePending(targetID, requesterID)
	if err != nil {
		return nil, err
	}

	return request, nil
}

// ApproveAdminDeletion approves a Pending request and executes the
// deletion of the target Admin account (session revocation + soft
// delete). The four-eyes rule, the target-role restriction and the
// last-active-Admin guard are all re-checked at decision time so a role
// change between request and decision can never open an escalation path.
func (s *UserService) ApproveAdminDeletion(
	deciderUserID, requestID string,
) error {
	deciderUserID = strings.TrimSpace(deciderUserID)
	requestID = strings.TrimSpace(requestID)

	deciderID, err := uuid.Parse(deciderUserID)
	if err != nil {
		return ErrInvalidUserID
	}

	if s.adminDeletionRepo == nil {
		return fmt.Errorf("admin deletion workflow is not configured")
	}

	decider, err := s.userRepo.FindByID(deciderUserID)
	if err != nil {
		return err
	}
	if !isAuthorizedAdminDecider(decider.Role.Name) {
		return ErrAdminDeletionNotAuthorized
	}

	request, err := s.adminDeletionRepo.FindByID(requestID)
	if err != nil {
		return err
	}
	if request.Status != models.AdminDeletionStatusPending {
		return ErrAdminDeletionRequestInvalid
	}
	if request.RequestedByID == deciderID {
		// Four-eyes rule: the requester can never decide their own request.
		return ErrAdminDeletionApprovalRequired
	}
	if request.TargetUserID == deciderID {
		return ErrCannotDeleteSelf
	}

	target, err := s.userRepo.FindByID(request.TargetUserID.String())
	if err != nil {
		return err
	}
	if target.Role.Name != models.RoleAdmin {
		return ErrAdminDeletionRequestInvalid
	}

	// Last-active-Admin guard, re-checked at decision time.
	activeAdmins, err := s.userRepo.CountActiveAdminsExcept(target.ID)
	if err != nil {
		return err
	}
	if activeAdmins == 0 {
		return ErrLastActiveAdmin
	}

	if err := s.adminDeletionRepo.MarkApproved(request.ID, deciderID); err != nil {
		return err
	}

	if err := s.sessionRepo.RevokeAllByUserID(target.ID.String()); err != nil {
		return err
	}

	if err := s.userRepo.SoftDelete(target); err != nil {
		return err
	}

	return nil
}

// RejectAdminDeletion rejects a Pending request. The target account is
// never touched; the same separation rules as approval apply.
func (s *UserService) RejectAdminDeletion(
	deciderUserID, requestID string,
) error {
	deciderUserID = strings.TrimSpace(deciderUserID)
	requestID = strings.TrimSpace(requestID)

	deciderID, err := uuid.Parse(deciderUserID)
	if err != nil {
		return ErrInvalidUserID
	}

	if s.adminDeletionRepo == nil {
		return fmt.Errorf("admin deletion workflow is not configured")
	}

	decider, err := s.userRepo.FindByID(deciderUserID)
	if err != nil {
		return err
	}
	if !isAuthorizedAdminDecider(decider.Role.Name) {
		return ErrAdminDeletionNotAuthorized
	}

	request, err := s.adminDeletionRepo.FindByID(requestID)
	if err != nil {
		return err
	}
	if request.Status != models.AdminDeletionStatusPending {
		return ErrAdminDeletionRequestInvalid
	}
	if request.RequestedByID == deciderID {
		return ErrAdminDeletionApprovalRequired
	}
	if request.TargetUserID == deciderID {
		return ErrCannotDeleteSelf
	}

	return s.adminDeletionRepo.MarkRejected(request.ID, deciderID)
}
