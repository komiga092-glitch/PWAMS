package repository

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

var ErrAuditLogNotFound = errors.New("audit log not found")

type AuditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{
		db: db,
	}
}

func (r *AuditLogRepository) Create(
	auditLog *models.AuditLog,
) error {
	return r.db.Create(auditLog).Error
}

func (r *AuditLogRepository) FindByID(
	id uuid.UUID,
) (*models.AuditLog, error) {
	var auditLog models.AuditLog

	err := r.db.
		Where("id = ?", id).
		First(&auditLog).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAuditLogNotFound
	}

	if err != nil {
		return nil, err
	}

	return &auditLog, nil
}

func (r *AuditLogRepository) List(
	query models.AuditLogListQuery,
) ([]models.AuditLog, int64, int, int, error) {
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

	dbQuery := r.db.Model(&models.AuditLog{})

	if query.Action != "" {
		dbQuery = dbQuery.Where(
			"action = ?",
			query.Action,
		)
	}

	if query.Entity != "" {
		dbQuery = dbQuery.Where(
			"entity = ?",
			query.Entity,
		)
	}

	if query.UserID != "" {
		userID, err := uuid.Parse(query.UserID)
		if err != nil {
			return nil, 0, 0, 0, err
		}

		dbQuery = dbQuery.Where(
			"user_id = ?",
			userID,
		)
	}

	var total int64

	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	offset := (page - 1) * pageSize

	var auditLogs []models.AuditLog

	if err := dbQuery.
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&auditLogs).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	totalPages := 0

	if total > 0 {
		totalPages = int(
			(total + int64(pageSize) - 1) /
				int64(pageSize),
		)
	}

	return auditLogs, total, page, totalPages, nil
}

func (r *AuditLogRepository) Delete(
	id uuid.UUID,
) error {
	result := r.db.
		Delete(&models.AuditLog{}, "id = ?", id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrAuditLogNotFound
	}

	return nil
}
