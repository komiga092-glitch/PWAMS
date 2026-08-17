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
	ErrInvalidLoanRepayment = errors.New(
		"invalid loan repayment",
	)

	ErrInvalidLoanRepaymentID = errors.New(
		"invalid loan repayment id",
	)

	ErrInvalidLoanIDFormat = errors.New(
		"invalid loan id",
	)

	ErrInvalidRepaymentAmount = errors.New(
		"invalid repayment amount",
	)

	ErrInvalidInstallmentNumber = errors.New(
		"invalid installment number",
	)

	ErrRepaymentAlreadyPaid = errors.New(
		"repayment already paid",
	)

	ErrRepaymentAmountTooHigh = errors.New(
		"repayment amount cannot exceed installment amount",
	)

	ErrInvalidRepaymentStatus = errors.New(
		"invalid repayment status",
	)

	ErrRepaymentCannotBeCancelled = errors.New(
		"repayment cannot be cancelled",
	)
)

type LoanRepaymentService struct {
	repaymentRepo *repository.LoanRepaymentRepository
	loanRepo      *repository.LoanRepository
}

func NewLoanRepaymentService(
	repaymentRepo *repository.LoanRepaymentRepository,
	loanRepo *repository.LoanRepository,
) *LoanRepaymentService {
	return &LoanRepaymentService{
		repaymentRepo: repaymentRepo,
		loanRepo:      loanRepo,
	}
}

func (s *LoanRepaymentService) Create(
	request models.CreateLoanRepaymentRequest,
) (*models.LoanRepayment, error) {
	loanID := strings.TrimSpace(request.LoanID)

	parsedLoanID, err := uuid.Parse(loanID)
	if err != nil {
		return nil, ErrInvalidLoanIDFormat
	}

	if request.InstallmentNumber <= 0 {
		return nil, ErrInvalidInstallmentNumber
	}

	if request.Amount <= 0 {
		return nil, ErrInvalidRepaymentAmount
	}

	loan, err := s.loanRepo.FindByID(loanID)
	if err != nil {
		return nil, err
	}

	if loan.ID != parsedLoanID {
		return nil, ErrInvalidLoanIDFormat
	}

	existing, err := s.repaymentRepo.FindByLoanAndInstallment(
		parsedLoanID,
		request.InstallmentNumber,
	)

	if err == nil && existing != nil {
		return nil, ErrInvalidLoanRepayment
	}

	if err != nil &&
		!errors.Is(err, repository.ErrLoanRepaymentNotFound) {
		return nil, err
	}

	dueDate, err := parseDateValue(request.DueDate)
	if err != nil {
		return nil, errors.New(
			"invalid repayment due date",
		)
	}

	repayment := &models.LoanRepayment{
		ID:                uuid.New(),
		LoanID:            parsedLoanID,
		InstallmentNumber: request.InstallmentNumber,
		DueDate:           dueDate,
		Amount:            request.Amount,
		PaidAmount:        0,
		Status:            models.RepaymentStatusPending,
		Notes:             strings.TrimSpace(request.Notes),
	}

	if err := s.repaymentRepo.Create(repayment); err != nil {
		return nil, err
	}

	return repayment, nil
}

func (s *LoanRepaymentService) GetByID(
	id string,
) (*models.LoanRepayment, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidLoanRepaymentID
	}

	return s.repaymentRepo.FindByID(id)
}

func (s *LoanRepaymentService) List(
	query models.LoanRepaymentListQuery,
) ([]models.LoanRepayment, int64, int, int, error) {
	query.LoanID = strings.TrimSpace(query.LoanID)
	query.Status = strings.TrimSpace(query.Status)

	if query.LoanID != "" {
		if _, err := uuid.Parse(query.LoanID); err != nil {
			return nil, 0, 0, 0, ErrInvalidLoanIDFormat
		}
	}

	if query.Status != "" {
		switch query.Status {
		case models.RepaymentStatusPending,
			models.RepaymentStatusPaid,
			models.RepaymentStatusOverdue,
			models.RepaymentStatusCancelled:
		default:
			return nil, 0, 0, 0, ErrInvalidRepaymentStatus
		}
	}

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

	repayments, total, err := s.repaymentRepo.List(query)
	if err != nil {
		return nil, 0, page, pageSize, err
	}

	return repayments, total, page, pageSize, nil
}

func (s *LoanRepaymentService) Pay(
	id string,
	request models.PayLoanRepaymentRequest,
) (*models.LoanRepayment, error) {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidLoanRepaymentID
	}

	if request.PaidAmount <= 0 {
		return nil, ErrInvalidRepaymentAmount
	}

	repayment, err := s.repaymentRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if repayment.Status == models.RepaymentStatusPaid {
		return nil, ErrRepaymentAlreadyPaid
	}

	if repayment.Status == models.RepaymentStatusCancelled {
		return nil, ErrRepaymentCannotBeCancelled
	}

	remainingAmount := repayment.Amount - repayment.PaidAmount

	if request.PaidAmount > remainingAmount {
		return nil, ErrRepaymentAmountTooHigh
	}

	repayment.PaidAmount += request.PaidAmount

	if strings.TrimSpace(request.PaymentReference) != "" {
		repayment.PaymentReference =
			strings.TrimSpace(request.PaymentReference)
	}

	if strings.TrimSpace(request.Notes) != "" {
		repayment.Notes =
			strings.TrimSpace(request.Notes)
	}

	if repayment.PaidAmount >= repayment.Amount {
		now := time.Now().UTC()

		repayment.PaidAmount = repayment.Amount
		repayment.Status = models.RepaymentStatusPaid
		repayment.PaidAt = &now
	}

	if err := s.repaymentRepo.Update(repayment); err != nil {
		return nil, err
	}

	// Check whether this loan has any remaining
	// pending or overdue repayments.
	if repayment.Status == models.RepaymentStatusPaid {
		hasOutstanding, err :=
			s.repaymentRepo.HasOutstandingRepayments(
				repayment.LoanID,
			)
		if err != nil {
			return nil, err
		}

		// If there are no remaining repayments,
		// mark the loan as completed.
		if !hasOutstanding {
			loan, err := s.loanRepo.FindByID(
				repayment.LoanID.String(),
			)
			if err != nil {
				return nil, err
			}

			if loan.Status == models.LoanStatusActive {
				now := time.Now().UTC()

				loan.Status = models.LoanStatusCompleted
				loan.CompletedAt = &now

				if err := s.loanRepo.Update(loan); err != nil {
					return nil, err
				}
			}
		}
	}

	return repayment, nil
}

func (s *LoanRepaymentService) MarkOverdue() error {
	now := time.Now().UTC()

	repayments, _, err := s.repaymentRepo.List(
		models.LoanRepaymentListQuery{
			Status:   models.RepaymentStatusPending,
			Page:     1,
			PageSize: 1000,
		},
	)
	if err != nil {
		return err
	}

	for i := range repayments {
		repayment := &repayments[i]

		if repayment.DueDate.Before(now) {
			repayment.Status = models.RepaymentStatusOverdue

			if err := s.repaymentRepo.Update(repayment); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *LoanRepaymentService) Cancel(
	id string,
) error {
	id = strings.TrimSpace(id)

	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidLoanRepaymentID
	}

	repayment, err := s.repaymentRepo.FindByID(id)
	if err != nil {
		return err
	}

	if repayment.Status == models.RepaymentStatusPaid {
		return ErrRepaymentCannotBeCancelled
	}

	repayment.Status = models.RepaymentStatusCancelled

	return s.repaymentRepo.Update(repayment)
}
