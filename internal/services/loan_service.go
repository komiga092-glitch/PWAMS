package services

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrInvalidLoanID               = errors.New("invalid loan id")
	ErrInvalidLoanAmount           = errors.New("loan amount must be greater than zero")
	ErrInvalidInterestRate         = errors.New("interest rate cannot be negative")
	ErrInvalidLoanDuration         = errors.New("loan duration must be greater than zero")
	ErrInvalidLoanStatus           = errors.New("invalid loan status")
	ErrInvalidLoanStatusTransition = errors.New("invalid loan status transition")
	ErrLoanCannotBeEdited          = errors.New("loan cannot be edited in its current status")
)

type LoanService struct {
	loanRepo   *repository.LoanRepository
	personRepo *repository.PersonRepository
}

func NewLoanService(
	loanRepo *repository.LoanRepository,
	personRepo *repository.PersonRepository,
) *LoanService {
	return &LoanService{
		loanRepo:   loanRepo,
		personRepo: personRepo,
	}
}

func (s *LoanService) CreateLoan(
	request models.CreateLoanRequest,
	createdByID uuid.UUID,
) (*models.Loan, error) {

	personID, err := uuid.Parse(strings.TrimSpace(request.PersonID))
	if err != nil {
		return nil, ErrInvalidPersonID
	}

	person, err := s.personRepo.FindByID(request.PersonID)
	if err != nil {
		return nil, err
	}

	if person.Status != models.PersonStatusActive {
		return nil, errors.New("person account is not active")
	}

	if request.LoanAmount <= 0 {
		return nil, ErrInvalidLoanAmount
	}

	if request.InterestRate < 0 {
		return nil, ErrInvalidInterestRate
	}

	if request.DurationMonths <= 0 {
		return nil, ErrInvalidLoanDuration
	}

	installment := calculateLoanInstallment(
		request.LoanAmount,
		request.InterestRate,
		request.DurationMonths,
	)

	loan := &models.Loan{
		ID:                uuid.New(),
		PersonID:          personID,
		LoanAmount:        request.LoanAmount,
		InterestRate:      request.InterestRate,
		DurationMonths:    request.DurationMonths,
		InstallmentAmount: installment,
		Status:            models.LoanStatusPending,
		Purpose:           strings.TrimSpace(request.Purpose),
		CreatedByID:       createdByID,
	}

	if err := s.loanRepo.Create(loan); err != nil {
		return nil, err
	}

	loan.Person = *person

	return loan, nil
}

func calculateLoanInstallment(
	amount float64,
	interestRate float64,
	durationMonths int,
) float64 {
	totalInterest := amount * (interestRate / 100)
	totalAmount := amount + totalInterest

	return totalAmount / float64(durationMonths)
}

func (s *LoanService) GetLoanByID(
	id string,
) (*models.Loan, error) {

	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidLoanID
	}

	return s.loanRepo.FindByID(id)
}

func (s *LoanService) ListLoans(
	query models.LoanListQuery,
) ([]models.Loan, int64, int, int, error) {

	page := query.Page
	if page < 1 {
		page = 1
	}

	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	query.Page = page
	query.PageSize = pageSize

	loans, total, err := s.loanRepo.List(query)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return loans, total, page, pageSize, nil
}

func (s *LoanService) ReviewLoan(
	id string,
	request models.ReviewLoanRequest,
	reviewerID uuid.UUID,
) (*models.Loan, error) {

	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidLoanID
	}

	loan, err := s.loanRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	newStatus := strings.TrimSpace(request.Status)

	if !isValidLoanStatus(newStatus) {
		return nil, ErrInvalidLoanStatus
	}

	if !isValidLoanStatusTransition(
		loan.Status,
		newStatus,
	) {
		return nil, ErrInvalidLoanStatusTransition
	}

	now := time.Now().UTC()

	loan.Status = newStatus

	if newStatus == models.LoanStatusApproved {
		loan.ApprovedByID = &reviewerID
		loan.ApprovedAt = &now
	}

	if newStatus == models.LoanStatusActive {
		loan.DisbursedAt = &now
	}

	if newStatus == models.LoanStatusCompleted {
		loan.CompletedAt = &now
	}

	if err := s.loanRepo.Update(loan); err != nil {
		return nil, err
	}

	return loan, nil
}

func isValidLoanStatus(status string) bool {
	switch status {
	case models.LoanStatusPending,
		models.LoanStatusApproved,
		models.LoanStatusRejected,
		models.LoanStatusActive,
		models.LoanStatusCompleted,
		models.LoanStatusCancelled:
		return true
	default:
		return false
	}
}

func isValidLoanStatusTransition(
	currentStatus string,
	newStatus string,
) bool {
	switch currentStatus {

	case models.LoanStatusPending:
		return newStatus == models.LoanStatusApproved ||
			newStatus == models.LoanStatusRejected ||
			newStatus == models.LoanStatusCancelled

	case models.LoanStatusApproved:
		return newStatus == models.LoanStatusActive ||
			newStatus == models.LoanStatusCancelled

	case models.LoanStatusActive:
		return newStatus == models.LoanStatusCompleted ||
			newStatus == models.LoanStatusCancelled

	default:
		return false
	}
}
