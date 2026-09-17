package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

// Sync support requirements:
// 1. Normal Person queries must exclude is_deleted = true records.
// 2. Delete must use soft delete semantics by setting is_deleted = true.
// 3. Sync updates must preserve optimistic-lock versioning.
// 4. Every successful update must increment version.
// 5. updated_at must be updated on every mutation.
// 6. updated_by must be stored when the modifying user is available.
// 7. Do not physically delete Person records.
// 8. Preserve all existing repository behavior and method signatures unless a change is required for sync support.

var (
	ErrPersonNotFound = errors.New("person not found")
)

type PersonRepository struct {
	db *gorm.DB
}

func NewPersonRepository(db *gorm.DB) *PersonRepository {
	return &PersonRepository{
		db: db,
	}
}

func (r *PersonRepository) ExistsByNICPassport(
	nicPassport string,
) (bool, error) {
	var count int64

	value := strings.ToLower(strings.TrimSpace(nicPassport))

	err := r.db.
		Model(&models.Person{}).
		Where("LOWER(nic_passport) = ?", value).
		Count(&count).
		Error

	if err != nil {
		return false, fmt.Errorf(
			"failed to check existing person: %w",
			err,
		)
	}

	return count > 0, nil
}

func (r *PersonRepository) Create(person *models.Person) error {
	if err := r.db.Create(person).Error; err != nil {
		return fmt.Errorf("failed to create person: %w", err)
	}

	return nil
}

func (r *PersonRepository) List(
	search string,
	status string,
	page int,
	pageSize int,
	ownerID uuid.UUID,
) ([]models.Person, int64, error) {
	var persons []models.Person
	var total int64

	query := r.db.Model(&models.Person{})

	// Object-level scoping: non-privileged callers receive their own user
	// ID so they only ever see persons they created; privileged roles
	// receive uuid.Nil (unrestricted).
	if ownerID != uuid.Nil {
		query = query.Where("created_by_id = ?", ownerID)
	}

	if search != "" {
		searchValue := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"

		query = query.Where(
			`LOWER(full_name) LIKE ?
			OR LOWER(nic_passport) LIKE ?
			OR LOWER(phone) LIKE ?
			OR LOWER(email) LIKE ?`,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
		)
	}

	if status != "" {
		query = query.Where(
			"LOWER(status) = LOWER(?)",
			strings.TrimSpace(status),
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count persons: %w", err)
	}

	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 20
	}

	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	if err := query.
		Preload("CreatedBy").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&persons).
		Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list persons: %w", err)
	}

	return persons, total, nil
}

func (r *PersonRepository) FindByID(id string) (*models.Person, error) {
	var person models.Person

	err := r.db.
		Preload("CreatedBy").
		First(&person, "id = ?", id).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPersonNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find person by id: %w", err)
	}

	return &person, nil
}

func (r *PersonRepository) ExistsByNICPassportExceptID(
	nicPassport, personID string,
) (bool, error) {
	var count int64

	value := strings.ToLower(strings.TrimSpace(nicPassport))

	err := r.db.
		Model(&models.Person{}).
		Where(
			"id <> ? AND LOWER(nic_passport) = ?",
			personID,
			value,
		).
		Count(&count).
		Error

	if err != nil {
		return false, fmt.Errorf(
			"failed to check duplicate NIC or passport: %w",
			err,
		)
	}

	return count > 0, nil
}
func (r *PersonRepository) Update(person *models.Person) error {
	if err := r.db.Save(person).Error; err != nil {
		return fmt.Errorf("failed to update person: %w", err)
	}

	return nil
}

func (r *PersonRepository) UpdateStatus(
	personID, status string,
) error {
	result := r.db.
		Model(&models.Person{}).
		Where("id = ?", personID).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf(
			"failed to update person status: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return ErrPersonNotFound
	}

	return nil
}

func (r *PersonRepository) SoftDelete(person *models.Person) error {
	if err := r.db.Delete(person).Error; err != nil {
		return fmt.Errorf("failed to delete person: %w", err)
	}

	return nil
}
