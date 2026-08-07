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
	ErrInvalidStudentID = errors.New("invalid student id")

	ErrInvalidStudentStatus = errors.New("selected student status is invalid")
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

func (s *StudentService) GetStudentByID(
	id string,
) (*models.Student, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidStudentID
	}

	student, err := s.studentRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	return student, nil
}

func isValidStudentStatus(status string) bool {
	switch status {
	case models.StudentStatusActive,
		models.StudentStatusInactive,
		models.StudentStatusPending:
		return true

	default:
		return false
	}
}

func (s *StudentService) UpdateStudent(
	id string,
	request models.UpdateStudentRequest,
) (*models.Student, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidStudentID
	}

	student, err := s.studentRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	personID := strings.TrimSpace(request.PersonID)

	parsedPersonID, err := uuid.Parse(personID)
	if err != nil {
		return nil, ErrInvalidPersonID
	}

	if _, err := s.personRepo.FindByID(personID); err != nil {
		return nil, err
	}

	studentCode := strings.ToUpper(
		strings.TrimSpace(request.StudentCode),
	)

	exists, err := s.studentRepo.ExistsByStudentCodeExceptID(
		studentCode,
		id,
	)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrStudentAlreadyExists
	}

	status := strings.TrimSpace(request.Status)

	if !isValidStudentStatus(status) {
		return nil, ErrInvalidStudentStatus
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

	student.PersonID = parsedPersonID
	student.FullName = strings.TrimSpace(request.FullName)
	student.SchoolName = strings.TrimSpace(request.SchoolName)
	student.Grade = strings.TrimSpace(request.Grade)
	student.StudentCode = studentCode
	student.DateOfBirth = dateOfBirth
	student.Gender = strings.TrimSpace(request.Gender)
	student.GuardianName = strings.TrimSpace(request.GuardianName)
	student.GuardianPhone = strings.TrimSpace(request.GuardianPhone)
	student.AcademicYear = request.AcademicYear
	student.Remarks = strings.TrimSpace(request.Remarks)
	student.Status = status

	if err := s.studentRepo.Update(student); err != nil {
		return nil, err
	}

	student.Person.ID = parsedPersonID

	return student, nil
}

func (s *StudentService) UpdateStudentStatus(
	id, status string,
) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidStudentID
	}

	if !isValidStudentStatus(status) {
		return ErrInvalidStudentStatus
	}

	if err := s.studentRepo.UpdateStatus(id, status); err != nil {
		return err
	}

	return nil
}

func (s *StudentService) DeleteStudent(id string) error {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidStudentID
	}

	student, err := s.studentRepo.FindByID(id)
	if err != nil {
		return err
	}

	if err := s.studentRepo.SoftDelete(student); err != nil {
		return err
	}

	return nil
}
