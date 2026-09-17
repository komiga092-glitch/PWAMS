package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

var ErrLoanRepaymentNotFound = errors.New("loan repayment not found")

type LoanRepaymentRepository struct {
	db *gorm.DB
}

func NewLoanRepaymentRepository(
	db *gorm.DB,
) *LoanRepaymentRepository {
	return &LoanRepaymentRepository{
		db: db,
	}
}

func (r *LoanRepaymentRepository) Create(
	repayment *models.LoanRepayment,
) error {
	return r.db.Create(repayment).Error
}

func (r *LoanRepaymentRepository) FindByID(
	id string,
) (*models.LoanRepayment, error) {
	var repayment models.LoanRepayment

	err := r.db.
		Preload("Loan").
		First(&repayment, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLoanRepaymentNotFound
	}

	if err != nil {
		return nil, err
	}

	return &repayment, nil
}

func (r *LoanRepaymentRepository) List(
	query models.LoanRepaymentListQuery,
	ownerID uuid.UUID,
) ([]models.LoanRepayment, int64, error) {
	var repayments []models.LoanRepayment
	var total int64

	db := r.db.Model(&models.LoanRepayment{})

	// Ownership is inherited through the parent loan: repayment rows are
	// scoped to the principal that created the loan they belong to.
	// Privileged callers receive uuid.Nil (unrestricted).
	if ownerID != uuid.Nil {
		db = db.
			Joins("JOIN loans ON loans.id = loan_repayments.loan_id").
			Where("loans.created_by_id = ?", ownerID)
	}

	if query.LoanID != "" {
		db = db.Where("loan_id = ?", query.LoanID)
	}

	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := query.Page
	if page < 1 {
		page = 1
	}

	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	err := db.
		Preload("Loan").
		Order("due_date ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&repayments).Error

	if err != nil {
		return nil, 0, err
	}

	return repayments, total, nil
}

func (r *LoanRepaymentRepository) FindByLoanAndInstallment(
	loanID uuid.UUID,
	installmentNumber int,
) (*models.LoanRepayment, error) {
	var repayment models.LoanRepayment

	err := r.db.
		Preload("Loan").
		Where(
			"loan_id = ? AND installment_number = ?",
			loanID,
			installmentNumber,
		).
		First(&repayment).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLoanRepaymentNotFound
	}

	if err != nil {
		return nil, err
	}

	return &repayment, nil
}

func (r *LoanRepaymentRepository) HasOutstandingRepayments(
	loanID uuid.UUID,
) (bool, error) {
	var count int64

	err := r.db.
		Model(&models.LoanRepayment{}).
		Where("loan_id = ?", loanID).
		Where(
			"status IN ?",
			[]string{
				models.RepaymentStatusPending,
				models.RepaymentStatusOverdue,
			},
		).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *LoanRepaymentRepository) Update(
	repayment *models.LoanRepayment,
) error {
	return r.db.Save(repayment).Error
}

// UpdatePaymentGuarded applies the payment mutation only when the row
// still carries the status and paid amount the caller observed. It is a
// compare-and-swap that prevents concurrent/double-payments from being
// silently merged (lost update).
func (r *LoanRepaymentRepository) UpdatePaymentGuarded(
	repayment *models.LoanRepayment,
	prevStatus string,
	prevPaidAmount decimal.Decimal,
) (bool, error) {
	result := r.db.
		Model(&models.LoanRepayment{}).
		Where(
			"id = ? AND status = ? AND paid_amount = ?",
			repayment.ID,
			prevStatus,
			prevPaidAmount,
		).
		Updates(map[string]interface{}{
			"status":            repayment.Status,
			"paid_amount":       repayment.PaidAmount,
			"paid_at":           repayment.PaidAt,
			"payment_reference": repayment.PaymentReference,
			"notes":             repayment.Notes,
		})

	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

func (r *LoanRepaymentRepository) Delete(
	id string,
) error {
	result := r.db.
		Delete(&models.LoanRepayment{}, "id = ?", id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrLoanRepaymentNotFound
	}

	return nil
}
