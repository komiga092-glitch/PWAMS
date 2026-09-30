package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

var (
	ErrUserAlreadyExists = errors.New("username or email already exists")
	ErrInvalidRole       = errors.New("selected role is invalid")
)

var ErrInvalidUserStatus = errors.New("selected user status is invalid")
var ErrCannotDeleteSelf = errors.New(
	"you cannot delete your own account",
)
var ErrCannotModifySuperAdmin = errors.New(
	"only a Super Admin can modify a Super Admin account",
)
var ErrActiveManagerExists = errors.New(
	"an active Manager already exists; deactivate the current Manager before assigning a new one",
)

// ErrLastActiveSuperAdmin, ErrLastActiveAdmin and the supervised Admin
// deletion errors are declared in user_admin_deletion.go (shared with the
// admin deletion workflow). ErrAdminDeletionRequiresApproval below is the
// service-level refusal for the generic delete path: an Admin actor deleting
// another Admin must use the supervised workflow instead.

var ErrAdminDeletionRequiresApproval = errors.New(
	"deleting an Admin account requires the supervised admin deletion approval workflow",
)

type UserService struct {
	userRepo    *repository.UserRepository
	roleRepo    *repository.RoleRepository
	sessionRepo *repository.SessionRepository
	// adminDeletionRepo backs the supervised Admin deletion workflow. It
	// is optional (variadic constructor argument) so existing callers and
	// tests that never exercise the workflow keep compiling unchanged.
	adminDeletionRepo *repository.AdminDeletionRequestRepository
}

func NewUserService(
	userRepo *repository.UserRepository,
	roleRepo *repository.RoleRepository,
	sessionRepo *repository.SessionRepository,
	adminDeletionRepo ...*repository.AdminDeletionRequestRepository,
) *UserService {
	service := &UserService{
		userRepo:    userRepo,
		roleRepo:    roleRepo,
		sessionRepo: sessionRepo,
	}

	if len(adminDeletionRepo) > 0 {
		service.adminDeletionRepo = adminDeletionRepo[0]
	}

	return service
}

// ensureManagerSlotAvailable enforces that no more than one Active user
// may hold the NGO Manager role at a time. Assigning or activating a
// Manager while another Active Manager exists is refused.
func (s *UserService) ensureManagerSlotAvailable(
	roleName string,
	status string,
	excludeUserID string,
) error {
	if roleName != models.RoleManager || status != models.UserStatusActive {
		return nil
	}
	count, err := s.userRepo.CountActiveByRoleName(models.RoleManager, excludeUserID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrActiveManagerExists
	}
	return nil
}

// isSuperAdminActor reports whether the actor role set grants the platform
// Super Admin capability. An empty or unknown role set fails closed: it is
// never treated as a Super Admin actor.
func isSuperAdminActor(actorRoles []string) bool {
	for _, role := range actorRoles {
		if strings.TrimSpace(role) == models.RoleSuperAdmin {
			return true
		}
	}
	return false
}

// ensureOtherActiveSuperAdminExists refuses any change that would leave the
// platform without an active Super Admin account (the target is excluded
// from the count, so a lone Super Admin is protected).
func (s *UserService) ensureOtherActiveSuperAdminExists(excludeUserID string) error {
	count, err := s.userRepo.CountActiveByRoleNames(
		[]string{models.RoleSuperAdmin},
		excludeUserID,
	)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrLastActiveSuperAdmin
	}
	return nil
}

// ensureOtherActiveProtectedAdminExists refuses any change that would leave
// the platform without an active protected admin (Admin or Super Admin).
func (s *UserService) ensureOtherActiveProtectedAdminExists(excludeUserID string) error {
	count, err := s.userRepo.CountActiveByRoleNames(
		[]string{models.RoleAdmin, models.RoleSuperAdmin},
		excludeUserID,
	)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrLastActiveAdmin
	}
	return nil
}

func (s *UserService) CreateUser(
	request models.CreateUserRequest,
	actorRoles ...string,
) (*models.User, error) {
	username := strings.ToLower(strings.TrimSpace(request.Username))
	email := strings.ToLower(strings.TrimSpace(request.Email))
	roleName := models.NormalizeRoleInput(request.Role)

	// Platform guard: only a Super Admin actor may mint a Super Admin
	// account. An empty/unknown actor role set fails closed.
	if roleName == models.RoleSuperAdmin && !isSuperAdminActor(actorRoles) {
		return nil, ErrCannotModifySuperAdmin
	}

	exists, err := s.userRepo.ExistsByUsernameOrEmail(username, email)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrUserAlreadyExists
	}

	role, err := s.roleRepo.FindByName(roleName)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrInvalidRole
		}

		return nil, err
	}

	if err := s.ensureManagerSlotAvailable(role.Name, models.UserStatusActive, ""); err != nil {
		return nil, err
	}

	passwordHash, err := utils.HashPassword(request.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		RoleID:       role.ID,
		Role:         *role,
		Status:       models.UserStatusActive,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) ListUsers(
	query models.UserListQuery,
) ([]models.User, int64, int, int, error) {
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

	users, total, err := s.userRepo.List(
		query.Search,
		query.Role,
		page,
		pageSize,
	)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return users, total, page, pageSize, nil
}

var ErrInvalidUserID = errors.New("invalid user id")
var ErrInvalidPassword = errors.New(
	"password must contain at least 8 characters",
)
var ErrCurrentPasswordIncorrect = errors.New("current password is incorrect")
var ErrPasswordsDoNotMatch = errors.New("new passwords do not match")

func (s *UserService) UpdateOwnProfile(
	userID uuid.UUID,
	request models.UpdateOwnProfileRequest,
) (*models.User, error) {
	username := strings.ToLower(strings.TrimSpace(request.Username))
	email := strings.ToLower(strings.TrimSpace(request.Email))
	exists, err := s.userRepo.ExistsByUsernameOrEmailExceptID(
		username,
		email,
		userID.String(),
	)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	user, err := s.userRepo.FindByID(userID.String())
	if err != nil {
		return nil, err
	}
	user.Username = username
	user.Email = email
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) ChangeOwnPassword(
	userID uuid.UUID,
	request models.ChangePasswordRequest,
) error {
	if request.NewPassword != request.ConfirmPassword {
		return ErrPasswordsDoNotMatch
	}
	if len(request.NewPassword) < 8 {
		return ErrInvalidPassword
	}

	user, err := s.userRepo.FindByID(userID.String())
	if err != nil {
		return err
	}
	if err := utils.CheckPassword(user.PasswordHash, request.CurrentPassword); err != nil {
		return ErrCurrentPasswordIncorrect
	}
	passwordHash, err := utils.HashPassword(request.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}
	return s.userRepo.UpdatePassword(userID.String(), passwordHash)
}

func (s *UserService) GetUserByID(id string) (*models.User, error) {
	id = strings.TrimSpace(id)

	// A malformed identifier and a well-formed UUID that matches no row are
	// the same outcome for a caller addressing GET /users/:id: there is no
	// such user, so the read answers 404 Not Found (QA USR-003) instead of
	// leaking identifier-format validation as a 400.
	if _, err := uuid.Parse(id); err != nil {
		return nil, repository.ErrUserNotFound
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func isValidUserStatus(status string) bool {
	switch status {
	case models.UserStatusActive,
		models.UserStatusDisabled,
		models.UserStatusLocked:
		return true

	default:
		return false
	}
}

func (s *UserService) UpdateUser(
	id string,
	request models.UpdateUserRequest,
	actorRoles ...string,
) (*models.User, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidUserID
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user.Role.Name == models.RoleSuperAdmin &&
		(len(actorRoles) == 0 || actorRoles[0] != models.RoleSuperAdmin) {
		return nil, ErrCannotModifySuperAdmin
	}

	username := strings.ToLower(strings.TrimSpace(request.Username))
	email := strings.ToLower(strings.TrimSpace(request.Email))
	roleName := models.NormalizeRoleInput(request.Role)
	status := strings.TrimSpace(request.Status)

	// Privilege-escalation guard: granting the Super Admin role requires a
	// Super Admin actor, no matter which account is being edited.
	if roleName == models.RoleSuperAdmin && !isSuperAdminActor(actorRoles) {
		return nil, ErrCannotModifySuperAdmin
	}

	if !isValidUserStatus(status) {
		return nil, ErrInvalidUserStatus
	}

	exists, err := s.userRepo.ExistsByUsernameOrEmailExceptID(
		username,
		email,
		id,
	)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrUserAlreadyExists
	}

	role, err := s.roleRepo.FindByName(roleName)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrInvalidRole
		}

		return nil, err
	}

	if err := s.ensureManagerSlotAvailable(role.Name, status, id); err != nil {
		return nil, err
	}

	// Last-active platform guards: the Super Admin set and the protected
	// Admin set (Admin + Super Admin) must never become empty through a
	// status change, demotion or role change of their last active member.
	if user.Role.Name == models.RoleSuperAdmin &&
		(role.Name != models.RoleSuperAdmin || status != models.UserStatusActive) {
		if err := s.ensureOtherActiveSuperAdminExists(id); err != nil {
			return nil, err
		}
	}
	if user.Role.Name == models.RoleAdmin &&
		((role.Name != models.RoleAdmin && role.Name != models.RoleSuperAdmin) ||
			status != models.UserStatusActive) {
		if err := s.ensureOtherActiveProtectedAdminExists(id); err != nil {
			return nil, err
		}
	}

	user.Username = username
	user.Email = email
	user.RoleID = role.ID
	user.Role = *role
	user.Status = status

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) UpdateUserStatus(
	id, status string,
	actorRoles ...string,
) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidUserID
	}

	if !isValidUserStatus(status) {
		return ErrInvalidUserStatus
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	// Platform guard: only a Super Admin actor may change the status of a
	// Super Admin account. An empty/unknown actor role set fails closed.
	if user.Role.Name == models.RoleSuperAdmin && !isSuperAdminActor(actorRoles) {
		return ErrCannotModifySuperAdmin
	}

	// Last-active platform guards for deactivation.
	if status != models.UserStatusActive {
		if user.Role.Name == models.RoleSuperAdmin {
			if err := s.ensureOtherActiveSuperAdminExists(id); err != nil {
				return err
			}
		}
		if user.Role.Name == models.RoleAdmin {
			if err := s.ensureOtherActiveProtectedAdminExists(id); err != nil {
				return err
			}
		}
	}

	if err := s.ensureManagerSlotAvailable(user.Role.Name, status, id); err != nil {
		return err
	}

	if err := s.userRepo.UpdateStatus(id, status); err != nil {
		return err
	}

	return nil
}

func (s *UserService) ResetPassword(
	id, newPassword string,
	actorRoles ...string,
) error {
	id = strings.TrimSpace(id)
	newPassword = strings.TrimSpace(newPassword)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidUserID
	}

	if len(newPassword) < 8 {
		return ErrInvalidPassword
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}
	if user.Role.Name == models.RoleSuperAdmin &&
		(len(actorRoles) == 0 || actorRoles[0] != models.RoleSuperAdmin) {
		return ErrCannotModifySuperAdmin
	}

	passwordHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(
		id,
		passwordHash,
	); err != nil {
		return err
	}

	return nil
}

func (s *UserService) DeleteUser(
	targetUserID, currentUserID string,
	actorRoles ...string,
) error {
	targetUserID = strings.TrimSpace(targetUserID)
	currentUserID = strings.TrimSpace(currentUserID)

	if _, err := uuid.Parse(targetUserID); err != nil {
		return ErrInvalidUserID
	}

	if targetUserID == currentUserID {
		return ErrCannotDeleteSelf
	}

	user, err := s.userRepo.FindByID(targetUserID)
	if err != nil {
		return err
	}

	actorIsSuperAdmin := isSuperAdminActor(actorRoles)

	// Platform guard: only a Super Admin actor may delete a Super Admin
	// account. An empty/unknown actor role set fails closed.
	if user.Role.Name == models.RoleSuperAdmin && !actorIsSuperAdmin {
		return ErrCannotModifySuperAdmin
	}

	// Two-person rule: an Admin actor deleting another Admin must go
	// through the supervised admin deletion approval workflow.
	if user.Role.Name == models.RoleAdmin && !actorIsSuperAdmin {
		return ErrAdminDeletionRequiresApproval
	}

	// Last-active platform guards.
	if user.Role.Name == models.RoleSuperAdmin {
		if err := s.ensureOtherActiveSuperAdminExists(targetUserID); err != nil {
			return err
		}
	}
	if user.Role.Name == models.RoleAdmin {
		if err := s.ensureOtherActiveProtectedAdminExists(targetUserID); err != nil {
			return err
		}
	}

	if err := s.sessionRepo.RevokeAllByUserID(targetUserID); err != nil {
		return err
	}

	if err := s.userRepo.SoftDelete(user); err != nil {
		return err
	}

	return nil
}
