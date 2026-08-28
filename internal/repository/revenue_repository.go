package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var ErrRevenueRecordNotFound = errors.New("revenue record not found")

type RevenueRepository struct{ db *gorm.DB }

func NewRevenueRepository(db *gorm.DB) *RevenueRepository { return &RevenueRepository{db: db} }

func (r *RevenueRepository) Create(record *models.RevenueRecord) error {
	return r.db.Create(record).Error
}

func (r *RevenueRepository) ReferenceExists(reference string, excludeID string) (bool, error) {
	if strings.TrimSpace(reference) == "" {
		return false, nil
	}
	query := r.db.Model(&models.RevenueRecord{}).Where("reference_no = ?", strings.TrimSpace(reference))
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *RevenueRepository) FindByID(id string) (*models.RevenueRecord, error) {
	var record models.RevenueRecord
	err := r.db.Preload("CreatedBy").First(&record, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRevenueRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find revenue record: %w", err)
	}
	return &record, nil
}

func (r *RevenueRepository) Update(record *models.RevenueRecord) error {
	return r.db.Save(record).Error
}

func (r *RevenueRepository) Delete(id string) error {
	result := r.db.Delete(&models.RevenueRecord{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRevenueRecordNotFound
	}
	return nil
}

func (r *RevenueRepository) List(query models.RevenueListQuery) ([]models.RevenueRecord, int64, error) {
	var records []models.RevenueRecord
	var total int64
	db := r.db.Model(&models.RevenueRecord{})
	if value := strings.TrimSpace(query.RecordType); value != "" {
		db = db.Where("record_type = ?", value)
	}
	if value := strings.TrimSpace(query.Category); value != "" {
		db = db.Where("category = ?", value)
	}
	if value := strings.TrimSpace(query.FromDate); value != "" {
		db = db.Where("record_date >= ?", value)
	}
	if value := strings.TrimSpace(query.ToDate); value != "" {
		db = db.Where("record_date < ?", value)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("CreatedBy").Order("record_date DESC").Limit(query.PageSize).Offset((query.Page - 1) * query.PageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *RevenueRepository) Summary(period string, start, end time.Time) (*models.RevenueSummary, error) {
	var result struct {
		Income   decimal.Decimal
		Expenses decimal.Decimal
	}
	err := r.db.Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN record_type = ? THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN record_type = ? THEN amount ELSE 0 END), 0) AS expenses
		FROM revenue_records
		WHERE deleted_at IS NULL AND record_date >= ? AND record_date < ?`,
		models.RevenueTypeIncome, models.RevenueTypeExpense, start, end).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return &models.RevenueSummary{Period: period, Income: result.Income, Expenses: result.Expenses, Net: result.Income.Sub(result.Expenses)}, nil
}
