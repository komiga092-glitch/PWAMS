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

type UserService struct {
	userRepo *repository.UserRepository
	roleRepo *repository.RoleRepository
}

func NewUserService(
	userRepo *repository.UserRepository,
	roleRepo *repository.RoleRepository,
) *UserService {
	return &UserService{
		userRepo: userRepo,
		roleRepo: roleRepo,
	}
}

func (s *UserService) CreateUser(
	request models.CreateUserRequest,
) (*models.User, error) {
	username := strings.ToLower(strings.TrimSpace(request.Username))
	email := strings.ToLower(strings.TrimSpace(request.Email))
	roleName := strings.TrimSpace(request.Role)

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

func (s *UserService) GetUserByID(id string) (*models.User, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidUserID
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
		models.UserStatusLocked,
		models.UserStatusPending:
		return true

	default:
		return false
	}
}

func (s *UserService) UpdateUser(
	id string,
	request models.UpdateUserRequest,
) (*models.User, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidUserID
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	username := strings.ToLower(strings.TrimSpace(request.Username))
	email := strings.ToLower(strings.TrimSpace(request.Email))
	roleName := strings.TrimSpace(request.Role)
	status := strings.TrimSpace(request.Status)

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
	id string,
	status string,
) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidUserID
	}

	if !isValidUserStatus(status) {
		return ErrInvalidUserStatus
	}

	if err := s.userRepo.UpdateStatus(id, status); err != nil {
		return err
	}

	return nil
}
