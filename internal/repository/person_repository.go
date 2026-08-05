package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

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
) ([]models.Person, int64, error) {
	var persons []models.Person
	var total int64

	query := r.db.Model(&models.Person{})

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
	nicPassport string,
	personID string,
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
