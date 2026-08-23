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
	ErrInvalidDonorType = errors.New("invalid donor type")

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

	ErrInvalidDonorStatus = errors.New(
		"selected donor status is invalid",
	)
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

// normalizeDonorType converts all supported donor type inputs
// into the application's canonical model values.
//
// Accepted:
//   - individual
//   - Individual
//   - INDIVIDUAL
//   - organization
//   - Organization
//   - ORGANIZATION
func normalizeDonorType(value string) (string, error) {
	value = strings.TrimSpace(value)

	switch strings.ToLower(value) {
	case "individual":
		return models.DonorTypeIndividual, nil

	case "organization":
		return models.DonorTypeOrganization, nil

	default:
		return "", ErrInvalidDonorType
	}
}

func isValidDonorStatus(status string) bool {
	status = strings.TrimSpace(status)

	switch status {
	case models.DonorStatusActive,
		models.DonorStatusInactive,
		models.DonorStatusPending:
		return true

	default:
		return false
	}
}

func (s *DonorService) CreateDonor(
	request models.CreateDonorRequest,
	createdByID uuid.UUID,
) (*models.Donor, error) {
	// =========================
	// NORMALIZE DONOR TYPE
	// =========================

	donorType, err := normalizeDonorType(
		request.DonorType,
	)
	if err != nil {
		return nil, err
	}

	nicPassport := strings.ToUpper(
		strings.TrimSpace(request.NICPassport),
	)

	registrationNumber := strings.ToUpper(
		strings.TrimSpace(request.RegistrationNumber),
	)

	organizationName := strings.TrimSpace(
		request.OrganizationName,
	)
	if !isValidPhone(strings.TrimSpace(request.Phone)) ||
		!isValidPhone(strings.TrimSpace(request.ContactPersonPhone)) {
		return nil, ErrInvalidPhone
	}

	// =========================
	// TYPE-SPECIFIC VALIDATION
	// =========================

	switch donorType {
	case models.DonorTypeIndividual:
		if nicPassport == "" {
			return nil, ErrIndividualDonorIdentityRequired
		}

		organizationName = ""
		registrationNumber = ""

	case models.DonorTypeOrganization:
		if organizationName == "" ||
			registrationNumber == "" {
			return nil, ErrOrganizationDetailsRequired
		}

		nicPassport = ""

	default:
		return nil, ErrInvalidDonorType
	}

	// =========================
	// DUPLICATE CHECK
	// =========================

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

	// =========================
	// CREATE DONOR
	// =========================

	donor := &models.Donor{
		Name: strings.TrimSpace(
			request.Name,
		),

		DonorType: donorType,

		NICPassport: nicPassport,

		OrganizationName: organizationName,

		RegistrationNumber: registrationNumber,

		Phone: strings.TrimSpace(
			request.Phone,
		),

		Email: strings.ToLower(
			strings.TrimSpace(request.Email),
		),

		Address: strings.TrimSpace(
			request.Address,
		),

		ContactPersonName: strings.TrimSpace(
			request.ContactPersonName,
		),

		ContactPersonPhone: strings.TrimSpace(
			request.ContactPersonPhone,
		),

		PreferredDonationType: strings.TrimSpace(
			request.PreferredDonationType,
		),

		Notes: strings.TrimSpace(
			request.Notes,
		),

		Status: models.DonorStatusActive,

		CreatedByID: createdByID,
	}

	if err := s.donorRepo.Create(
		donor,
	); err != nil {
		return nil, fmt.Errorf(
			"unable to create donor: %w",
			err,
		)
	}

	return donor, nil
}

func (s *DonorService) ListDonors(
	query models.DonorListQuery,
) ([]models.Donor, int64, int, int, error) {
	if strings.TrimSpace(query.Type) != "" {
		if _, err := normalizeDonorType(query.Type); err != nil {
			return nil, 0, 1, 10, err
		}
	}
	if strings.TrimSpace(query.Status) != "" &&
		!isValidDonorStatus(query.Status) {
		return nil, 0, 1, 10, ErrInvalidDonorStatus
	}

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

func (s *DonorService) UpdateDonor(
	id string,
	request models.UpdateDonorRequest,
) (*models.Donor, error) {
	// =========================
	// VALIDATE ID
	// =========================

	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidDonorID
	}

	// =========================
	// FIND DONOR
	// =========================

	donor, err := s.donorRepo.FindByID(id)

	if err != nil {
		return nil, err
	}

	// =========================
	// NORMALIZE TYPE
	// =========================

	donorType, err := normalizeDonorType(
		request.DonorType,
	)

	if err != nil {
		return nil, err
	}

	nicPassport := strings.ToUpper(
		strings.TrimSpace(request.NICPassport),
	)

	registrationNumber := strings.ToUpper(
		strings.TrimSpace(request.RegistrationNumber),
	)

	organizationName := strings.TrimSpace(
		request.OrganizationName,
	)

	status := strings.TrimSpace(
		request.Status,
	)
	if !isValidPhone(strings.TrimSpace(request.Phone)) ||
		!isValidPhone(strings.TrimSpace(request.ContactPersonPhone)) {
		return nil, ErrInvalidPhone
	}

	// =========================
	// TYPE-SPECIFIC VALIDATION
	// =========================

	switch donorType {
	case models.DonorTypeIndividual:
		if nicPassport == "" {
			return nil, ErrIndividualDonorIdentityRequired
		}

		// Individual donors must not keep
		// organization-specific fields.
		organizationName = ""
		registrationNumber = ""

	case models.DonorTypeOrganization:
		if organizationName == "" ||
			registrationNumber == "" {
			return nil, ErrOrganizationDetailsRequired
		}

		// Organization donors must not keep
		// individual identity field.
		nicPassport = ""

	default:
		return nil, ErrInvalidDonorType
	}

	// =========================
	// STATUS VALIDATION
	// =========================

	if !isValidDonorStatus(status) {
		return nil, ErrInvalidDonorStatus
	}

	// =========================
	// DUPLICATE CHECK
	// =========================

	exists, err := s.donorRepo.ExistsByIdentityExceptID(
		nicPassport,
		registrationNumber,
		id,
	)

	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrDonorAlreadyExists
	}

	// =========================
	// UPDATE DONOR
	// =========================

	donor.Name = strings.TrimSpace(
		request.Name,
	)

	donor.DonorType = donorType

	donor.NICPassport = nicPassport

	donor.OrganizationName = organizationName

	donor.RegistrationNumber = registrationNumber

	donor.Phone = strings.TrimSpace(
		request.Phone,
	)

	donor.Email = strings.ToLower(
		strings.TrimSpace(request.Email),
	)

	donor.Address = strings.TrimSpace(
		request.Address,
	)

	donor.ContactPersonName = strings.TrimSpace(
		request.ContactPersonName,
	)

	donor.ContactPersonPhone = strings.TrimSpace(
		request.ContactPersonPhone,
	)

	donor.PreferredDonationType = strings.TrimSpace(
		request.PreferredDonationType,
	)

	donor.Notes = strings.TrimSpace(
		request.Notes,
	)

	donor.Status = status

	// =========================
	// SAVE
	// =========================

	if err := s.donorRepo.Update(
		donor,
	); err != nil {
		return nil, err
	}

	return donor, nil
}

func (s *DonorService) UpdateDonorStatus(
	id string,
	status string,
) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidDonorID
	}

	if !isValidDonorStatus(status) {
		return ErrInvalidDonorStatus
	}

	if err := s.donorRepo.UpdateStatus(
		id,
		status,
	); err != nil {
		return err
	}

	return nil
}

func (s *DonorService) DeleteDonor(
	id string,
) error {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidDonorID
	}

	donor, err := s.donorRepo.FindByID(id)

	if err != nil {
		return err
	}

	if err := s.donorRepo.SoftDelete(
		donor,
	); err != nil {
		return err
	}

	return nil
}
