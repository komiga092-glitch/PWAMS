package repository

import (
	"errors"
	"strings"

	"github.com/google/uuid"

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
	ownerID uuid.UUID,
) ([]models.Loan, int64, error) {
	var loans []models.Loan
	var total int64

	db := r.db.Model(&models.Loan{})

	// Object-level scoping: non-privileged callers receive their own user
	// ID so they only ever see loans they created; privileged roles
	// receive uuid.Nil (unrestricted).
	if ownerID != uuid.Nil {
		db = db.Where("created_by_id = ?", ownerID)
	}

	if query.Search != "" {
		searchValue := "%" + strings.ToLower(strings.TrimSpace(query.Search)) + "%"
		db = db.Where(
			"LOWER(CAST(id AS TEXT)) LIKE ? OR LOWER(CAST(person_id AS TEXT)) LIKE ? OR LOWER(purpose) LIKE ?",
			searchValue,
			searchValue,
			searchValue,
		)
	}

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
