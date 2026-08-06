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
)

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
