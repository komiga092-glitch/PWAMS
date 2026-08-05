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
	ErrPersonAlreadyExists = errors.New(
		"a person with this NIC or passport already exists",
	)
	ErrInvalidDateOfBirth = errors.New("invalid date of birth")
)
var ErrInvalidPersonID = errors.New("invalid person id")

var ErrInvalidPersonStatus = errors.New(
	"selected person status is invalid",
)

type PersonService struct {
	personRepo *repository.PersonRepository
}

func NewPersonService(
	personRepo *repository.PersonRepository,
) *PersonService {
	return &PersonService{
		personRepo: personRepo,
	}
}

func (s *PersonService) CreatePerson(
	request models.CreatePersonRequest,
	createdByID uuid.UUID,
) (*models.Person, error) {
	fullName := strings.TrimSpace(request.FullName)
	nicPassport := strings.ToUpper(
		strings.TrimSpace(request.NICPassport),
	)

	exists, err := s.personRepo.ExistsByNICPassport(nicPassport)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrPersonAlreadyExists
	}

	var dateOfBirth *time.Time

	if strings.TrimSpace(request.DateOfBirth) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			request.DateOfBirth,
		)
		if err != nil {
			return nil, ErrInvalidDateOfBirth
		}

		if parsedDate.After(time.Now()) {
			return nil, ErrInvalidDateOfBirth
		}

		dateOfBirth = &parsedDate
	}

	person := &models.Person{
		FullName:      fullName,
		NICPassport:   nicPassport,
		DateOfBirth:   dateOfBirth,
		Gender:        strings.TrimSpace(request.Gender),
		Phone:         strings.TrimSpace(request.Phone),
		Email:         strings.ToLower(strings.TrimSpace(request.Email)),
		Address:       strings.TrimSpace(request.Address),
		Occupation:    strings.TrimSpace(request.Occupation),
		MonthlyIncome: request.MonthlyIncome,
		Status:        models.PersonStatusActive,
		CreatedByID:   createdByID,
	}

	if err := s.personRepo.Create(person); err != nil {
		return nil, fmt.Errorf("unable to create person: %w", err)
	}

	return person, nil
}

func (s *PersonService) ListPersons(
	query models.PersonListQuery,
) ([]models.Person, int64, int, int, error) {
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

	persons, total, err := s.personRepo.List(
		query.Search,
		query.Status,
		page,
		pageSize,
	)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return persons, total, page, pageSize, nil
}

func (s *PersonService) GetPersonByID(id string) (*models.Person, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidPersonID
	}

	person, err := s.personRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	return person, nil
}

func isValidPersonStatus(status string) bool {
	switch status {
	case models.PersonStatusActive,
		models.PersonStatusInactive,
		models.PersonStatusPending:
		return true

	default:
		return false
	}
}
func (s *PersonService) UpdatePerson(
	id string,
	request models.UpdatePersonRequest,
) (*models.Person, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidPersonID
	}

	person, err := s.personRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	fullName := strings.TrimSpace(request.FullName)
	nicPassport := strings.ToUpper(
		strings.TrimSpace(request.NICPassport),
	)
	status := strings.TrimSpace(request.Status)

	if !isValidPersonStatus(status) {
		return nil, ErrInvalidPersonStatus
	}

	exists, err := s.personRepo.ExistsByNICPassportExceptID(
		nicPassport,
		id,
	)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrPersonAlreadyExists
	}

	var dateOfBirth *time.Time

	if strings.TrimSpace(request.DateOfBirth) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			request.DateOfBirth,
		)
		if err != nil {
			return nil, ErrInvalidDateOfBirth
		}

		if parsedDate.After(time.Now()) {
			return nil, ErrInvalidDateOfBirth
		}

		dateOfBirth = &parsedDate
	}

	person.FullName = fullName
	person.NICPassport = nicPassport
	person.DateOfBirth = dateOfBirth
	person.Gender = strings.TrimSpace(request.Gender)
	person.Phone = strings.TrimSpace(request.Phone)
	person.Email = strings.ToLower(strings.TrimSpace(request.Email))
	person.Address = strings.TrimSpace(request.Address)
	person.Occupation = strings.TrimSpace(request.Occupation)
	person.MonthlyIncome = request.MonthlyIncome
	person.Status = status

	if err := s.personRepo.Update(person); err != nil {
		return nil, err
	}

	return person, nil
}

func (s *PersonService) UpdatePersonStatus(
	id string,
	status string,
) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidPersonID
	}

	if !isValidPersonStatus(status) {
		return ErrInvalidPersonStatus
	}

	if err := s.personRepo.UpdateStatus(id, status); err != nil {
		return err
	}

	return nil
}

func (s *PersonService) DeletePerson(id string) error {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidPersonID
	}

	person, err := s.personRepo.FindByID(id)
	if err != nil {
		return err
	}

	if err := s.personRepo.SoftDelete(person); err != nil {
		return err
	}

	return nil
}
