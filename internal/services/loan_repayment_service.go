package services

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

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

	ErrInvalidRepaymentDueDate = errors.New(
		"invalid repayment due date",
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

	ErrLoanNotPayable = errors.New(
		"payments are only allowed on active loans",
	)

	ErrRepaymentModified = errors.New(
		"repayment was modified concurrently, please retry",
	)
)

type LoanRepaymentService struct {
	repaymentRepo *repository.LoanRepaymentRepository
	loanRepo      *repository.LoanRepository
	db            *gorm.DB
}

func NewLoanRepaymentService(
	repaymentRepo *repository.LoanRepaymentRepository,
	loanRepo *repository.LoanRepository,
	db *gorm.DB,
) *LoanRepaymentService {
	return &LoanRepaymentService{
		repaymentRepo: repaymentRepo,
		loanRepo:      loanRepo,
		db:            db,
	}
}

// isDuplicateKeyError reports whether the database rejected the write
// because of a unique constraint violation (Postgres SQLSTATE 23505).
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "23505") ||
		strings.Contains(message, "duplicate key")
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

	if request.Amount.LessThanOrEqual(decimal.Zero) {
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
		return nil, ErrInvalidRepaymentDueDate
	}

	repayment := &models.LoanRepayment{
		ID:                uuid.New(),
		LoanID:            parsedLoanID,
		InstallmentNumber: request.InstallmentNumber,
		DueDate:           dueDate,
		Amount:            request.Amount,
		PaidAmount:        decimal.Zero,
		Status:            models.RepaymentStatusPending,
		Notes:             strings.TrimSpace(request.Notes),
	}

	if err := s.repaymentRepo.Create(repayment); err != nil {
		// The composite unique index is the authoritative guard; map a
		// lost uniqueness race to the same conflict as the pre-check.
		if isDuplicateKeyError(err) {
			return nil, ErrInvalidLoanRepayment
		}
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
	if pageSize > 100 {
		pageSize = 100
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

	if request.PaidAmount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidRepaymentAmount
	}

	var repayment *models.LoanRepayment

	// Payment application and any resulting loan closure form one
	// business unit and must commit atomically (B4). Transaction-scoped
	// repositories keep repository access patterns identical to the
	// non-transactional paths while sharing the caller's transaction.
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txRepayments := repository.NewLoanRepaymentRepository(tx)
		txLoans := repository.NewLoanRepository(tx)

		current, err := txRepayments.FindByID(id)
		if err != nil {
			return err
		}
		repayment = current

		switch repayment.Status {
		case models.RepaymentStatusPaid:
			return ErrRepaymentAlreadyPaid

		case models.RepaymentStatusCancelled:
			return ErrRepaymentCannotBeCancelled
		}

		// A repayment may only be settled against a loan that has actually
		// been disbursed (Active). Payments against Pending, Rejected or
		// Cancelled loans are business-invalid.
		loan, err := txLoans.FindByID(repayment.LoanID.String())
		if err != nil {
			return err
		}

		if loan.Status != models.LoanStatusActive {
			return ErrLoanNotPayable
		}

		remainingAmount := repayment.Amount.Sub(repayment.PaidAmount)

		if request.PaidAmount.GreaterThan(remainingAmount) {
			return ErrRepaymentAmountTooHigh
		}

		prevStatus := repayment.Status
		prevPaidAmount := repayment.PaidAmount

		repayment.PaidAmount = repayment.PaidAmount.Add(request.PaidAmount)

		if strings.TrimSpace(request.PaymentReference) != "" {
			repayment.PaymentReference =
				strings.TrimSpace(request.PaymentReference)
		}

		if strings.TrimSpace(request.Notes) != "" {
			repayment.Notes =
				strings.TrimSpace(request.Notes)
		}

		if repayment.PaidAmount.GreaterThanOrEqual(repayment.Amount) {
			now := time.Now().UTC()

			repayment.PaidAmount = repayment.Amount
			repayment.Status = models.RepaymentStatusPaid
			repayment.PaidAt = &now
		}

		// Compare-and-swap write: if another request concurrently recorded
		// a payment against this installment, the guarded update affects
		// zero rows and we surface a conflict instead of silently losing
		// money. Any partial work rolls back with the transaction.
		updated, err := txRepayments.UpdatePaymentGuarded(
			repayment,
			prevStatus,
			prevPaidAmount,
		)
		if err != nil {
			return err
		}

		if !updated {
			return ErrRepaymentModified
		}

		// If there are no remaining repayments, mark the loan completed
		// inside the same atomic unit.
		if repayment.Status == models.RepaymentStatusPaid {
			hasOutstanding, err :=
				txRepayments.HasOutstandingRepayments(
					repayment.LoanID,
				)
			if err != nil {
				return err
			}

			if !hasOutstanding && loan.Status == models.LoanStatusActive {
				now := time.Now().UTC()

				loan.Status = models.LoanStatusCompleted
				loan.CompletedAt = &now

				if err := txLoans.Update(loan); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if txErr != nil {
		return nil, txErr
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
