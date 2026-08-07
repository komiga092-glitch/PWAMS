package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/constants"
)

var (
	ErrInvalidDonationType = errors.New("invalid donation type")

	ErrCashAmountRequired = errors.New(
		"amount must be greater than zero for cash donations",
	)

	ErrItemDetailsRequired = errors.New(
		"item name, quantity and unit are required for item donations",
	)

	ErrDonationReferenceExists = errors.New(
		"donation reference number already exists",
	)

	ErrInvalidDonationDate = errors.New(
		"invalid donation date",
	)

	ErrDonorNotActive = errors.New(
		"donor account is not active",
	)

	ErrInvalidDonationDateRange = errors.New(
		"invalid donation date range",
	)

	ErrInvalidDonationStatus = errors.New(
		"invalid donation status",
	)
	ErrInvalidDonationID = errors.New("invalid donation id")
)
var ErrConfirmedDonationCannotDelete = errors.New(
	"confirmed donations cannot be deleted",
)

func isValidDonationStatus(status string) bool {
	switch status {
	case models.DonationStatusPending,
		models.DonationStatusConfirmed,
		models.DonationStatusCancelled:
		return true

	default:
		return false
	}
}

type DonationService struct {
	donationRepo *repository.DonationRepository
	donorRepo    *repository.DonorRepository
	personRepo   *repository.PersonRepository
}

func NewDonationService(
	donationRepo *repository.DonationRepository,
	donorRepo *repository.DonorRepository,
	personRepo *repository.PersonRepository,
) *DonationService {
	return &DonationService{
		donationRepo: donationRepo,
		donorRepo:    donorRepo,
		personRepo:   personRepo,
	}
}

func (s *DonationService) CreateDonation(
	request models.CreateDonationRequest,
	createdByID uuid.UUID,
) (*models.Donation, error) {
	parsedDonorID, err := s.validateDonor(request.DonorID)
if err != nil {
	return nil, err
}

	parsedPersonID, err := s.validatePerson(request.PersonID)
if err != nil {
	return nil, err
}

	donationType, err := s.validateDonationDetails(request)
if err != nil {
	return nil, err
}

	donationDate, err := s.parseDonationDate(request.DonationDate)
if err != nil {
	return nil, err
}

	referenceNo, err := s.generateReferenceNo(request.ReferenceNo)
if err != nil {
	return nil, err
}

	currency := normalizeCurrency(request.Currency)

	if currency == "" {
		currency = "LKR"
	}

	donation := &models.Donation{
		DonorID:      parsedDonorID,
		PersonID:     &parsedPersonID,
		DonationType: donationType,
		Amount:       request.Amount,
		Currency:     currency,
		ItemName:     strings.TrimSpace(request.ItemName),
		Quantity:     request.Quantity,
		Unit:         strings.TrimSpace(request.Unit),
		Description:  strings.TrimSpace(request.Description),
		DonationDate: donationDate,
		ReferenceNo:  referenceNo,
		Status:       models.DonationStatusPending,
		CreatedByID:  createdByID,
	}

	if donationType == models.DonationTypeCash {
		donation.ItemName = ""
		donation.Quantity = 0
		donation.Unit = ""
	}

	if err := s.donationRepo.Create(donation); err != nil {
		return nil, fmt.Errorf(
			"unable to create donation: %w",
			err,
		)
	}

	return donation, nil
}

func (s *DonationService) validateDonor(donorID string) (uuid.UUID, error) {
	donorID = strings.TrimSpace(donorID)

	parsedID, err := uuid.Parse(donorID)
	if err != nil {
		return uuid.Nil, ErrInvalidDonorID
	}

	donor, err := s.donorRepo.FindByID(donorID)
	if err != nil {
		return uuid.Nil, err
	}

	if donor.Status != models.DonorStatusActive {
		return uuid.Nil, ErrDonorNotActive
	}

	return parsedID, nil
}

func (s *DonationService) validatePerson(personID string) (uuid.UUID, error) {
	personID = strings.TrimSpace(personID)

	parsedID, err := uuid.Parse(personID)
	if err != nil {
		return uuid.Nil, ErrInvalidPersonID
	}

	if _, err := s.personRepo.FindByID(personID); err != nil {
		return uuid.Nil, err
	}

	return parsedID, nil
}

func (s *DonationService) validateDonationDetails(
	request models.CreateDonationRequest,
) (string, error) {

	donationType := strings.TrimSpace(request.DonationType)

	if !isValidDonationType(donationType) {
		return "", ErrInvalidDonationType
	}

	switch donationType {

	case models.DonationTypeCash:

		if request.Amount <= 0 {
			return "", ErrCashAmountRequired
		}

	default:

		if strings.TrimSpace(request.ItemName) == "" ||
			request.Quantity <= 0 ||
			strings.TrimSpace(request.Unit) == "" {

			return "", ErrItemDetailsRequired
		}

	}

	return donationType, nil
}
func isValidDonationType(value string) bool {
	switch value {
	case models.DonationTypeCash,
		models.DonationTypeFood,
		models.DonationTypeMedical,
		models.DonationTypeEducation,
		models.DonationTypeClothing,
		models.DonationTypeEquipment,
		models.DonationTypeOther:
		return true

	default:
		return false
	}
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func generateDonationReference() string {
	return fmt.Sprintf(
		"DON-%s-%s",
		time.Now().UTC().Format("20060102"),
		strings.ToUpper(uuid.NewString()[:6]),
	)
}

func (s *DonationService) ListDonations(
	query models.DonationListQuery,
) ([]models.Donation, int64, int, int, error) {
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

	donorID := strings.TrimSpace(query.DonorID)

	if donorID != "" {
		if _, err := uuid.Parse(donorID); err != nil {
			return nil, 0, page, pageSize, ErrInvalidDonorID
		}
	}

	personID := strings.TrimSpace(query.PersonID)

	if personID != "" {
		if _, err := uuid.Parse(personID); err != nil {
			return nil, 0, page, pageSize, ErrInvalidPersonID
		}
	}

	donationType := strings.TrimSpace(query.Type)

	if donationType != "" &&
		!isValidDonationType(donationType) {
		return nil, 0, page, pageSize, ErrInvalidDonationType
	}

	status := strings.TrimSpace(query.Status)

	if status != "" &&
		!isValidDonationStatus(status) {
		return nil, 0, page, pageSize, ErrInvalidDonationStatus
	}

	var fromDate *time.Time
	var toDate *time.Time

	if strings.TrimSpace(query.FromDate) != "" {
		parsed, err := time.Parse(
			"2006-01-02",
			query.FromDate,
		)
		if err != nil {
			return nil, 0, page, pageSize,
				ErrInvalidDonationDateRange
		}

		fromDate = &parsed
	}

	if strings.TrimSpace(query.ToDate) != "" {
		parsed, err := time.Parse(
			"2006-01-02",
			query.ToDate,
		)
		if err != nil {
			return nil, 0, page, pageSize,
				ErrInvalidDonationDateRange
		}

		toDate = &parsed
	}

	if fromDate != nil &&
		toDate != nil &&
		fromDate.After(*toDate) {
		return nil, 0, page, pageSize,
			ErrInvalidDonationDateRange
	}

	donations, total, err := s.donationRepo.List(
		query.Search,
		donorID,
		personID,
		donationType,
		status,
		fromDate,
		toDate,
		page,
		pageSize,
	)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return donations, total, page, pageSize, nil
}

func (s *DonationService) GetDonationByID(
	id string,
) (*models.Donation, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidDonationID
	}

	donation, err := s.donationRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	return donation, nil
}

func (s *DonationService) UpdateDonation(
	id string,
	request models.UpdateDonationRequest,
) (*models.Donation, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidDonationID
	}

	donation, err := s.donationRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	var personID *uuid.UUID

	if strings.TrimSpace(request.PersonID) != "" {
		value, err := uuid.Parse(strings.TrimSpace(request.PersonID))
		if err != nil {
			return nil, ErrInvalidPersonID
		}

		if _, err := s.personRepo.FindByID(value.String()); err != nil {
			return nil, err
		}

		personID = &value
	}

	donationType := strings.TrimSpace(request.DonationType)
	if !isValidDonationType(donationType) {
		return nil, ErrInvalidDonationType
	}

	status := strings.TrimSpace(request.Status)
	if !isValidDonationStatus(status) {
		return nil, ErrInvalidDonationStatus
	}

	switch donationType {
	case models.DonationTypeCash:
		if request.Amount <= 0 {
			return nil, ErrCashAmountRequired
		}

	default:
		if strings.TrimSpace(request.ItemName) == "" ||
			request.Quantity <= 0 ||
			strings.TrimSpace(request.Unit) == "" {
			return nil, ErrItemDetailsRequired
		}
	}

	donationDate := donation.DonationDate

	if strings.TrimSpace(request.DonationDate) != "" {
		parsedDate, err := time.Parse(
			"2006-01-02",
			request.DonationDate,
		)
		if err != nil {
			return nil, ErrInvalidDonationDate
		}

		if parsedDate.After(time.Now().UTC()) {
			return nil, ErrInvalidDonationDate
		}

		donationDate = parsedDate
	}

	currency := strings.ToUpper(strings.TrimSpace(request.Currency))
	if currency == "" {
		currency = "LKR"
	}

	donation.PersonID = personID
	donation.DonationType = donationType
	donation.Amount = request.Amount
	donation.Currency = currency
	donation.ItemName = strings.TrimSpace(request.ItemName)
	donation.Quantity = request.Quantity
	donation.Unit = strings.TrimSpace(request.Unit)
	donation.Description = strings.TrimSpace(request.Description)
	donation.DonationDate = donationDate
	donation.Status = status

	if donationType == models.DonationTypeCash {
		donation.ItemName = ""
		donation.Quantity = 0
		donation.Unit = ""
	} else {
		donation.Amount = 0
	}

	if err := s.donationRepo.Update(donation); err != nil {
		return nil, err
	}

	return donation, nil
}
func (s *DonationService) UpdateDonationStatus(
	id string,
	status string,
) error {
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidDonationID
	}

	if !isValidDonationStatus(status) {
		return ErrInvalidDonationStatus
	}

	if err := s.donationRepo.UpdateStatus(id, status); err != nil {
		return err
	}

	return nil
}

func (s *DonationService) DeleteDonation(id string) error {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidDonationID
	}

	donation, err := s.donationRepo.FindByID(id)
	if err != nil {
		return err
	}

	if donation.Status == models.DonationStatusConfirmed {
		return ErrConfirmedDonationCannotDelete
	}

	if err := s.donationRepo.SoftDelete(donation); err != nil {
		return fmt.Errorf("failed to soft delete donation: %w", err)
	}

	return nil
}
func (s *DonationService) parseDonationDate(date string) (time.Time, error) {

	if strings.TrimSpace(date) == "" {
		return time.Now().UTC(), nil
	}

	parsedDate, err := time.Parse(constants.DateLayout, date)
	if err != nil {
		return time.Time{}, ErrInvalidDonationDate
	}

	if parsedDate.After(time.Now().UTC()) {
		return time.Time{}, ErrInvalidDonationDate
	}

	return parsedDate, nil
}

func (s *DonationService) generateReferenceNo(reference string) (string, error) {

	reference = strings.ToUpper(strings.TrimSpace(reference))

	if reference == "" {
		reference = generateDonationReference()
	}

	exists, err := s.donationRepo.ExistsByReferenceNo(reference)
	if err != nil {
		return "", err
	}

	if exists {
		return "", ErrDonationReferenceExists
	}

	return reference, nil
}