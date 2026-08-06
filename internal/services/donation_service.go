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
	donorID := strings.TrimSpace(request.DonorID)

	parsedDonorID, err := uuid.Parse(donorID)
	if err != nil {
		return nil, ErrInvalidDonorID
	}

	donor, err := s.donorRepo.FindByID(donorID)
	if err != nil {
		return nil, err
	}

	if donor.Status != models.DonorStatusActive {
		return nil, ErrDonorNotActive
	}

	var parsedPersonID *uuid.UUID

	if strings.TrimSpace(request.PersonID) != "" {
		personID := strings.TrimSpace(request.PersonID)

		value, err := uuid.Parse(personID)
		if err != nil {
			return nil, ErrInvalidPersonID
		}

		if _, err := s.personRepo.FindByID(personID); err != nil {
			return nil, err
		}

		parsedPersonID = &value
	}

	donationType := strings.TrimSpace(request.DonationType)

	if !isValidDonationType(donationType) {
		return nil, ErrInvalidDonationType
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

	donationDate := time.Now().UTC()

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

	referenceNo := strings.ToUpper(
		strings.TrimSpace(request.ReferenceNo),
	)

	if referenceNo == "" {
		referenceNo = generateDonationReference()
	}

	exists, err := s.donationRepo.ExistsByReferenceNo(referenceNo)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrDonationReferenceExists
	}

	currency := strings.ToUpper(
		strings.TrimSpace(request.Currency),
	)

	if currency == "" {
		currency = "LKR"
	}

	donation := &models.Donation{
		DonorID:      parsedDonorID,
		PersonID:     parsedPersonID,
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
