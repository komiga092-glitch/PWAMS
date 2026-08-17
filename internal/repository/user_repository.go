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

const userIDCondition = "id = ?"

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
		Where(userIDCondition, userID).
		Update("last_login_at", now).
		Error; err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

func (r *UserRepository) ExistsByUsernameOrEmail(
	username, email string,
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
	search, role string,
	page, pageSize int,
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
		First(&user, userIDCondition, id).
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
	username, email, userID string,
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
	id string,
	status string,
) error {
	var user models.User

	err := r.db.
		Unscoped().
		Where("id = ?", id).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}

		return err
	}

	user.Status = status

	if user.DeletedAt.Valid {
		user.DeletedAt = gorm.DeletedAt{}
	}

	return r.db.
		Unscoped().
		Model(&user).
		Updates(map[string]interface{}{
			"status":     status,
			"deleted_at": nil,
			"updated_at": time.Now().UTC(),
		}).Error
}

func (r *UserRepository) UpdatePassword(
	userID, passwordHash string,
) error {
	result := r.db.
		Model(&models.User{}).
		Where(userIDCondition, userID).
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

func (r *UserRepository) SoftDelete(user *models.User) error {
	if err := r.db.Delete(user).Error; err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (r *UserRepository) CountAll() (int64, error) {
	var count int64

	if err := r.db.
		Model(&models.User{}).
		Count(&count).
		Error; err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	err := r.db.
		Preload("Role").
		Where("LOWER(email) = ?", normalizedEmail).
		First(&user).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	return &user, nil
}

type RoleUserCount struct {
	RoleName string
	Count    int64
}

func (r *UserRepository) CountByRole() (
	map[string]int64,
	error,
) {
	var results []RoleUserCount

	err := r.db.
		Table("users").
		Select(
			"roles.name AS role_name, COUNT(users.id) AS count",
		).
		Joins(
			"JOIN roles ON roles.id = users.role_id",
		).
		Where("users.deleted_at IS NULL").
		Group("roles.name").
		Scan(&results).
		Error

	if err != nil {
		return nil, fmt.Errorf(
			"failed to count users by role: %w",
			err,
		)
	}

	counts := make(map[string]int64)

	for _, result := range results {
		counts[result.RoleName] = result.Count
	}

	return counts, nil
}

func (r *UserRepository) CountByStatus(status string) (int64, error) {
	var count int64

	err := r.db.
		Model(&models.User{}).
		Where("status = ?", status).
		Count(&count).
		Error

	if err != nil {
		return 0, fmt.Errorf(
			"failed to count users by status: %w",
			err,
		)
	}

	return count, nil
}
