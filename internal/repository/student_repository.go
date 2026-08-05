package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var (
	ErrStudentNotFound = errors.New("student not found")
)

type StudentRepository struct {
	db *gorm.DB
}

func NewStudentRepository(db *gorm.DB) *StudentRepository {
	return &StudentRepository{
		db: db,
	}
}

func (r *StudentRepository) ExistsByStudentCode(
	studentCode string,
) (bool, error) {
	if strings.TrimSpace(studentCode) == "" {
		return false, nil
	}

	var count int64

	value := strings.ToUpper(strings.TrimSpace(studentCode))

	err := r.db.
		Model(&models.Student{}).
		Where("UPPER(student_code) = ?", value).
		Count(&count).
		Error

	if err != nil {
		return false, fmt.Errorf(
			"failed to check existing student code: %w",
			err,
		)
	}

	return count > 0, nil
}

func (r *StudentRepository) Create(student *models.Student) error {
	if err := r.db.Create(student).Error; err != nil {
		return fmt.Errorf("failed to create student: %w", err)
	}

	return nil
}
