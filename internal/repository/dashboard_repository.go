package repository

import (
	"fmt"

	"gorm.io/gorm"
)

type DashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

func (r *DashboardRepository) Count(table, where string, args ...any) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", table)
	if where != "" {
		query += " AND " + where
	}
	if err := r.db.Raw(query, args...).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *DashboardRepository) CountWithoutDeleted(table, where string, args ...any) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if where != "" {
		query += " WHERE " + where
	}
	if err := r.db.Raw(query, args...).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *DashboardRepository) Sum(table, column, where string, args ...any) (float64, error) {
	var total float64
	query := fmt.Sprintf("SELECT COALESCE(SUM(%s), 0) FROM %s WHERE deleted_at IS NULL", column, table)
	if where != "" {
		query += " AND " + where
	}
	if err := r.db.Raw(query, args...).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *DashboardRepository) NetRevenue() (float64, error) {
	var total float64
	err := r.db.Raw(`SELECT COALESCE(SUM(CASE WHEN record_type = 'income' THEN amount ELSE -amount END), 0) FROM revenue_records WHERE deleted_at IS NULL AND is_deleted = FALSE`).Scan(&total).Error
	return total, err
}
