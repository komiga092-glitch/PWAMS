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
