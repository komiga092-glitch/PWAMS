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

func (r *StudentRepository) List(
	search string,
	school string,
	grade string,
	status string,
	personID string,
	page int,
	pageSize int,
) ([]models.Student, int64, error) {
	var students []models.Student
	var total int64

	query := r.db.Model(&models.Student{})

	if search != "" {
		searchValue := "%" +
			strings.ToLower(strings.TrimSpace(search)) +
			"%"

		query = query.Where(
			`LOWER(full_name) LIKE ?
			OR LOWER(student_code) LIKE ?
			OR LOWER(guardian_name) LIKE ?
			OR LOWER(guardian_phone) LIKE ?`,
			searchValue,
			searchValue,
			searchValue,
			searchValue,
		)
	}

	if school != "" {
		query = query.Where(
			"LOWER(school_name) LIKE ?",
			"%"+strings.ToLower(strings.TrimSpace(school))+"%",
		)
	}

	if grade != "" {
		query = query.Where(
			"LOWER(grade) = LOWER(?)",
			strings.TrimSpace(grade),
		)
	}

	if status != "" {
		query = query.Where(
			"LOWER(status) = LOWER(?)",
			strings.TrimSpace(status),
		)
	}

	if personID != "" {
		query = query.Where(
			"person_id = ?",
			strings.TrimSpace(personID),
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to count students: %w",
			err,
		)
	}

	offset := (page - 1) * pageSize

	if err := query.
		Preload("Person").
		Preload("CreatedBy").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&students).
		Error; err != nil {
		return nil, 0, fmt.Errorf(
			"failed to list students: %w",
			err,
		)
	}

	return students, total, nil
}

func (r *StudentRepository) FindByID(id string) (*models.Student, error) {
	var student models.Student

	err := r.db.
		Preload("Person").
		Preload("CreatedBy").
		First(&student, "id = ?", id).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrStudentNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find student by id: %w", err)
	}

	return &student, nil
}

func (r *StudentRepository) ExistsByStudentCodeExceptID(
	studentCode, studentID string,
) (bool, error) {
	if strings.TrimSpace(studentCode) == "" {
		return false, nil
	}

	var count int64

	value := strings.ToUpper(strings.TrimSpace(studentCode))

	err := r.db.
		Model(&models.Student{}).
		Where(
			"id <> ? AND UPPER(student_code) = ?",
			studentID,
			value,
		).
		Count(&count).
		Error

	if err != nil {
		return false, fmt.Errorf(
			"failed to check duplicate student code: %w",
			err,
		)
	}

	return count > 0, nil
}

func (r *StudentRepository) Update(student *models.Student) error {
	if err := r.db.Save(student).Error; err != nil {
		return fmt.Errorf("failed to update student: %w", err)
	}

	return nil
}

func (r *StudentRepository) UpdateStatus(
	studentID string,
	status string,
) error {
	result := r.db.
		Model(&models.Student{}).
		Where("id = ?", studentID).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf(
			"failed to update student status: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return ErrStudentNotFound
	}

	return nil
}

func (r *StudentRepository) SoftDelete(
	student *models.Student,
) error {
	if err := r.db.Delete(student).Error; err != nil {
		return fmt.Errorf("failed to delete student: %w", err)
	}

	return nil
}
