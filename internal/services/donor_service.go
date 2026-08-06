package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidDonorType   = errors.New("invalid donor type")
	ErrDonorAlreadyExists = errors.New(
		"a donor with this NIC, passport, or registration number already exists",
	)
	ErrIndividualDonorIdentityRequired = errors.New(
		"NIC or passport is required for an individual donor",
	)
	ErrOrganizationDetailsRequired = errors.New(
		"organization name and registration number are required",
	)
	ErrInvalidDonorID = errors.New("invalid donor id")
)

type DonorService struct {
	donorRepo *repository.DonorRepository
}

func NewDonorService(
	donorRepo *repository.DonorRepository,
) *DonorService {
	return &DonorService{
		donorRepo: donorRepo,
	}
}

func (s *DonorService) CreateDonor(
	request models.CreateDonorRequest,
	createdByID uuid.UUID,
) (*models.Donor, error) {
	donorType := strings.TrimSpace(request.DonorType)
	nicPassport := strings.ToUpper(strings.TrimSpace(request.NICPassport))
	registrationNumber := strings.ToUpper(
		strings.TrimSpace(request.RegistrationNumber),
	)

	switch donorType {
	case models.DonorTypeIndividual:
		if nicPassport == "" {
			return nil, ErrIndividualDonorIdentityRequired
		}

	case models.DonorTypeOrganization:
		if strings.TrimSpace(request.OrganizationName) == "" ||
			registrationNumber == "" {
			return nil, ErrOrganizationDetailsRequired
		}

	default:
		return nil, ErrInvalidDonorType
	}

	exists, err := s.donorRepo.ExistsByIdentity(
		nicPassport,
		registrationNumber,
	)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrDonorAlreadyExists
	}

	donor := &models.Donor{
		Name:                  strings.TrimSpace(request.Name),
		DonorType:             donorType,
		NICPassport:           nicPassport,
		OrganizationName:      strings.TrimSpace(request.OrganizationName),
		RegistrationNumber:    registrationNumber,
		Phone:                 strings.TrimSpace(request.Phone),
		Email:                 strings.ToLower(strings.TrimSpace(request.Email)),
		Address:               strings.TrimSpace(request.Address),
		ContactPersonName:     strings.TrimSpace(request.ContactPersonName),
		ContactPersonPhone:    strings.TrimSpace(request.ContactPersonPhone),
		PreferredDonationType: strings.TrimSpace(request.PreferredDonationType),
		Notes:                 strings.TrimSpace(request.Notes),
		Status:                models.DonorStatusActive,
		CreatedByID:           createdByID,
	}

	if err := s.donorRepo.Create(donor); err != nil {
		return nil, fmt.Errorf("unable to create donor: %w", err)
	}

	return donor, nil
}

func (s *DonorService) ListDonors(
	query models.DonorListQuery,
) ([]models.Donor, int64, int, int, error) {
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

	donors, total, err := s.donorRepo.List(
		query.Search,
		query.Type,
		query.Status,
		page,
		pageSize,
	)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return donors, total, page, pageSize, nil
}

func (s *DonorService) GetDonorByID(
	id string,
) (*models.Donor, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidDonorID
	}

	donor, err := s.donorRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	return donor, nil
}
