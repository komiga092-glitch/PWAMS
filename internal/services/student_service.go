package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrStudentAlreadyExists = errors.New(
		"a student with this student code already exists",
	)
	ErrInvalidStudentDateOfBirth = errors.New(
		"invalid student date of birth",
	)
)

type StudentService struct {
	studentRepo *repository.StudentRepository
	personRepo  *repository.PersonRepository
}

func NewStudentService(
	studentRepo *repository.StudentRepository,
	personRepo *repository.PersonRepository,
) *StudentService {
	return &StudentService{
		studentRepo: studentRepo,
		personRepo:  personRepo,
	}
}

func (s *StudentService) CreateStudent(
	request models.CreateStudentRequest,
	createdByID uuid.UUID,
) (*models.Student, error) {
	personID := strings.TrimSpace(request.PersonID)

	parsedPersonID, err := uuid.Parse(personID)
	if err != nil {
		return nil, ErrInvalidPersonID
	}

	_, err = s.personRepo.FindByID(personID)
	if err != nil {
		return nil, err
	}

	studentCode := strings.ToUpper(
		strings.TrimSpace(request.StudentCode),
	)

	exists, err := s.studentRepo.ExistsByStudentCode(studentCode)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrStudentAlreadyExists
	}

	var dateOfBirth *time.Time

	if strings.TrimSpace(request.DateOfBirth) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			request.DateOfBirth,
		)
		if err != nil {
			return nil, ErrInvalidStudentDateOfBirth
		}

		if parsedDate.After(time.Now()) {
			return nil, ErrInvalidStudentDateOfBirth
		}

		dateOfBirth = &parsedDate
	}

	student := &models.Student{
		PersonID:      parsedPersonID,
		FullName:      strings.TrimSpace(request.FullName),
		SchoolName:    strings.TrimSpace(request.SchoolName),
		Grade:         strings.TrimSpace(request.Grade),
		StudentCode:   studentCode,
		DateOfBirth:   dateOfBirth,
		Gender:        strings.TrimSpace(request.Gender),
		GuardianName:  strings.TrimSpace(request.GuardianName),
		GuardianPhone: strings.TrimSpace(request.GuardianPhone),
		AcademicYear:  request.AcademicYear,
		Remarks:       strings.TrimSpace(request.Remarks),
		Status:        models.StudentStatusActive,
		CreatedByID:   createdByID,
	}

	if err := s.studentRepo.Create(student); err != nil {
		return nil, fmt.Errorf("unable to create student: %w", err)
	}

	return student, nil
}

func (s *StudentService) ListStudents(
	query models.StudentListQuery,
) ([]models.Student, int64, int, int, error) {
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

	if strings.TrimSpace(query.PersonID) != "" {
		if _, err := uuid.Parse(query.PersonID); err != nil {
			return nil, 0, page, pageSize, ErrInvalidPersonID
		}
	}

	students, total, err := s.studentRepo.List(
		query.Search,
		query.School,
		query.Grade,
		query.Status,
		query.PersonID,
		page,
		pageSize,
	)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return students, total, page, pageSize, nil
}
