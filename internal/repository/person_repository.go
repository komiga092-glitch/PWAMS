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
