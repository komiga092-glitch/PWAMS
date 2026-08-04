package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) FindByLogin(login string) (*models.User, error) {
	var user models.User

	normalizedLogin := strings.ToLower(strings.TrimSpace(login))

	err := r.db.
		Preload("Role").
		Where(
			"LOWER(username) = ? OR LOWER(email) = ?",
			normalizedLogin,
			normalizedLogin,
		).
		First(&user).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find user by login: %w", err)
	}

	return &user, nil

}

func (r *UserRepository) UpdateLastLogin(userID uuid.UUID) error {
	now := time.Now().UTC()

	if err := r.db.
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("last_login_at", now).
		Error; err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

func (r *UserRepository) ExistsByUsernameOrEmail(
	username string,
	email string,
) (bool, error) {
	var count int64

	err := r.db.
		Model(&models.User{}).
		Where(
			"LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)",
			username,
			email,
		).
		Count(&count).
		Error

	if err != nil {
		return false, fmt.Errorf("failed to check existing user: %w", err)
	}

	return count > 0, nil
}

func (r *UserRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *UserRepository) List(
	search string,
	role string,
	page int,
	pageSize int,
) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := r.db.
		Model(&models.User{}).
		Joins("JOIN roles ON roles.id = users.role_id")

	if search != "" {
		searchValue := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"

		query = query.Where(
			"LOWER(users.username) LIKE ? OR LOWER(users.email) LIKE ?",
			searchValue,
			searchValue,
		)
	}

	if role != "" {
		query = query.Where(
			"LOWER(roles.name) = LOWER(?)",
			strings.TrimSpace(role),
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	offset := (page - 1) * pageSize

	if err := query.
		Preload("Role").
		Order("users.created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&users).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

func (r *UserRepository) FindByID(id string) (*models.User, error) {
	var user models.User

	err := r.db.
		Preload("Role").
		First(&user, "id = ?", id).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) ExistsByUsernameOrEmailExceptID(
	username string,
	email string,
	userID string,
) (bool, error) {
	var count int64

	err := r.db.
		Model(&models.User{}).
		Where(
			"id <> ? AND (LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?))",
			userID,
			username,
			email,
		).
		Count(&count).
		Error

	if err != nil {
		return false, fmt.Errorf(
			"failed to check duplicate username or email: %w",
			err,
		)
	}

	return count > 0, nil
}

func (r *UserRepository) Update(user *models.User) error {
	if err := r.db.Save(user).Error; err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (r *UserRepository) UpdateStatus(
	userID string,
	status string,
) error {
	result := r.db.
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("failed to update user status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *UserRepository) UpdatePassword(
	userID string,
	passwordHash string,
) error {
	result := r.db.
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("password_hash", passwordHash)

	if result.Error != nil {
		return fmt.Errorf(
			"failed to update user password: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}
