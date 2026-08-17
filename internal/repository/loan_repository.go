package repository

import (
	"errors"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrLoanNotFound = errors.New("loan not found")

type LoanRepository struct {
	db *gorm.DB
}

func NewLoanRepository(db *gorm.DB) *LoanRepository {
	return &LoanRepository{
		db: db,
	}
}

func (r *LoanRepository) Create(loan *models.Loan) error {
	return r.db.Create(loan).Error
}

func (r *LoanRepository) FindByID(id string) (*models.Loan, error) {
	var loan models.Loan

	err := r.db.
		Preload("Person").
		Where("id = ?", id).
		First(&loan).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLoanNotFound
	}

	if err != nil {
		return nil, err
	}

	return &loan, nil
}

func (r *LoanRepository) List(
	query models.LoanListQuery,
) ([]models.Loan, int64, error) {
	var loans []models.Loan
	var total int64

	db := r.db.Model(&models.Loan{})

	if query.PersonID != "" {
		db = db.Where("person_id = ?", query.PersonID)
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

	if err := db.
		Preload("Person").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&loans).Error; err != nil {
		return nil, 0, err
	}

	return loans, total, nil
}

func (r *LoanRepository) Update(loan *models.Loan) error {
	return r.db.Save(loan).Error
}

func (r *LoanRepository) Delete(loan *models.Loan) error {
	return r.db.Delete(loan).Error
}
