package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

const (
	// These values should move to a settings table later per BR-5.
	minLoanAmount = 5000
	maxLoanAmount = 500000
)

var (
	ErrInvalidLoanID               = errors.New("invalid loan id")
	ErrInvalidLoanAmount           = errors.New("loan amount must be greater than zero")
	ErrInvalidInterestRate         = errors.New("interest rate cannot be negative")
	ErrInvalidLoanDuration         = errors.New("loan duration must be greater than zero")
	ErrInvalidLoanStatus           = errors.New("invalid loan status")
	ErrInvalidLoanStatusTransition = errors.New("invalid loan status transition")
	ErrLoanCannotBeEdited          = errors.New("loan cannot be edited in its current status")
	// ErrLoanAmountOutOfRange and ErrLoanDurationOutOfRange make the
	// create-time range rules classifiable errors, so a payload that breaks a
	// bound is answered as a validation failure instead of a 500 (QA LON-008
	// creates, LON-010 edits). The wrapped message keeps the exact bound and
	// the sentinel keeps errors.Is working.
	ErrLoanAmountOutOfRange   = errors.New("loan amount is outside the permitted range")
	ErrLoanDurationOutOfRange = errors.New("loan duration is outside the permitted range")
	// ErrPersonNotActive and ErrPersonHasActiveLoan are business-rule
	// rejections on create: they are client errors (422/409), never 500s.
	ErrPersonNotActive     = errors.New("person account is not active")
	ErrPersonHasActiveLoan = errors.New("person already has an active loan")
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
		return nil, ErrPersonNotActive
	}

	if request.LoanAmount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidLoanAmount
	}

	minLoanAmountDecimal := decimal.NewFromInt(int64(minLoanAmount))
	maxLoanAmountDecimal := decimal.NewFromInt(int64(maxLoanAmount))
	if request.LoanAmount.LessThan(minLoanAmountDecimal) {
		return nil, fmt.Errorf(
			"%w: loan amount must be at least LKR %s",
			ErrLoanAmountOutOfRange,
			minLoanAmountDecimal.String(),
		)
	}
	if request.LoanAmount.GreaterThan(maxLoanAmountDecimal) {
		return nil, fmt.Errorf(
			"%w: loan amount cannot exceed LKR %s",
			ErrLoanAmountOutOfRange,
			maxLoanAmountDecimal.String(),
		)
	}

	if request.InterestRate.LessThan(decimal.Zero) {
		return nil, ErrInvalidInterestRate
	}

	if request.DurationMonths <= 0 {
		return nil, ErrInvalidLoanDuration
	}
	if request.DurationMonths < 3 || request.DurationMonths > 36 {
		return nil, fmt.Errorf(
			"%w: loan duration must be between 3 and 36 months",
			ErrLoanDurationOutOfRange,
		)
	}

	activeCount, err := s.loanRepo.CountActiveByPersonID(request.PersonID)
	if err != nil {
		return nil, err
	}
	if activeCount > 0 {
		return nil, ErrPersonHasActiveLoan
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
	amount decimal.Decimal,
	interestRate decimal.Decimal,
	durationMonths int,
) decimal.Decimal {
	return CalculateLoanInstallmentPublic(amount, interestRate, durationMonths)
}

// CalculateLoanInstallmentPublic is exported for testing.
func CalculateLoanInstallmentPublic(
	amount decimal.Decimal,
	interestRate decimal.Decimal,
	durationMonths int,
) decimal.Decimal {
	totalInterest := amount.Mul(interestRate).Div(decimal.NewFromInt(100))
	totalAmount := amount.Add(totalInterest)
	return totalAmount.Div(decimal.NewFromInt(int64(durationMonths)))
}

func (s *LoanService) GetLoanByID(
	id string,
	actor Actor,
) (*models.Loan, error) {

	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidLoanID
	}

	loan, err := s.loanRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if !CanAccessRecord(actor, loan.CreatedByID) {
		return nil, ErrRecordAccessDenied
	}

	return loan, nil
}

func (s *LoanService) ListLoans(
	query models.LoanListQuery,
	actor Actor,
) ([]models.Loan, int64, int, int, error) {

	// Fail closed for unauthenticated/invalid actors before touching any
	// repository (the service may be constructed with nil repositories in
	// tests that assert this exact contract).
	ownerID, ok := ownershipFilter(actor)
	if !ok {
		return nil, 0, 0, 0, ErrRecordAccessDenied
	}

	page := query.Page
	if page < 1 {
		page = 1
	}

	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	if strings.TrimSpace(query.PersonID) != "" {
		if _, err := uuid.Parse(query.PersonID); err != nil {
			return nil, 0, page, pageSize, ErrInvalidPersonID
		}
	}

	query.Page = page
	query.PageSize = pageSize

	loans, total, err := s.loanRepo.List(query, ownerID)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return loans, total, page, pageSize, nil
}

func (s *LoanService) ReviewLoan(
	id string,
	request models.ReviewLoanRequest,
	reviewerID uuid.UUID,
	actor Actor,
) (*models.Loan, error) {

	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidLoanID
	}

	loan, err := s.loanRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if !CanAccessRecord(actor, loan.CreatedByID) {
		return nil, ErrRecordAccessDenied
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

	if newStatus == models.LoanStatusApproved || newStatus == models.LoanStatusRejected {
		if loan.CreatedByID == reviewerID {
			return nil, ErrCannotReviewOwnSubmission
		}
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

// UpdateLoan applies an edit to a loan that is still awaiting review. It is
// the consumer of models.UpdateLoanRequest (the counterpart of
// CreateLoanRequest) and reuses the create-time validation rules so an edit
// can never store terms a create would have refused; the installment is
// recomputed from the new terms so the schedule cannot drift from the
// principal, rate and duration (QA LON-009 / LON-010 / LON-011).
//
// Object-level authorization mirrors GetLoanByID/ListLoans: the submitter or a
// privileged role may edit, everybody else is denied. Only a loan that is
// still Pending may be changed (ErrLoanCannotBeEdited), and every rule is
// checked before the record is touched so a rejected payload leaves the stored
// loan exactly as it was.
func (s *LoanService) UpdateLoan(
	id string,
	request models.UpdateLoanRequest,
	actor Actor,
) (*models.Loan, error) {

	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidLoanID
	}

	loan, err := s.loanRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if !CanAccessRecord(actor, loan.CreatedByID) {
		return nil, ErrRecordAccessDenied
	}

	if loan.Status != models.LoanStatusPending {
		return nil, ErrLoanCannotBeEdited
	}

	if request.LoanAmount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidLoanAmount
	}

	minLoanAmountDecimal := decimal.NewFromInt(int64(minLoanAmount))
	maxLoanAmountDecimal := decimal.NewFromInt(int64(maxLoanAmount))

	if request.LoanAmount.LessThan(minLoanAmountDecimal) ||
		request.LoanAmount.GreaterThan(maxLoanAmountDecimal) {
		return nil, fmt.Errorf(
			"%w: loan amount must be between LKR %s and LKR %s",
			ErrLoanAmountOutOfRange,
			minLoanAmountDecimal.String(),
			maxLoanAmountDecimal.String(),
		)
	}

	if request.InterestRate.LessThan(decimal.Zero) {
		return nil, ErrInvalidInterestRate
	}

	if request.DurationMonths <= 0 {
		return nil, ErrInvalidLoanDuration
	}

	if request.DurationMonths < 3 || request.DurationMonths > 36 {
		return nil, fmt.Errorf(
			"%w: loan duration must be between 3 and 36 months",
			ErrLoanDurationOutOfRange,
		)
	}

	loan.LoanAmount = request.LoanAmount
	loan.InterestRate = request.InterestRate
	loan.DurationMonths = request.DurationMonths
	loan.InstallmentAmount = calculateLoanInstallment(
		request.LoanAmount,
		request.InterestRate,
		request.DurationMonths,
	)
	loan.Purpose = strings.TrimSpace(request.Purpose)

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
