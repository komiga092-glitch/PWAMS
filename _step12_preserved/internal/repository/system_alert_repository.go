package repository

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

var ErrSystemAlertNotFound = errors.New("system alert not found")

// SystemAlertRepository persists and queries the privileged system-alert
// inbox. Internal stable identifiers (alert_code, category, severity, source)
// are stored as-is; presentation happens through i18n keys.
type SystemAlertRepository struct {
	db *gorm.DB
}

func NewSystemAlertRepository(db *gorm.DB) *SystemAlertRepository {
	return &SystemAlertRepository{db: db}
}

func (r *SystemAlertRepository) Create(alert *models.SystemAlert) error {
	if alert == nil {
		return errors.New("system alert is nil")
	}
	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	return r.db.Create(alert).Error
}

// CreateOrIncrement records a new system alert or, when a still-open alert
// for the same (alert_code, source) pair already exists, increments its
// occurrence count and refreshes last_occurred_at (deduplication). The
// partial unique index uq_system_alerts_open_code_source makes the
// find-or-update race-safe at the database level.
func (r *SystemAlertRepository) CreateOrIncrement(alert *models.SystemAlert) error {
	if alert == nil {
		return errors.New("system alert is nil")
	}
	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}

	code := strings.TrimSpace(alert.AlertCode)
	source := strings.TrimSpace(alert.Source)

	var existing models.SystemAlert
	err := r.db.Where(
		"alert_code = ? AND source = ? AND status = ?",
		code, source, models.SystemAlertStatusOpen,
	).First(&existing).Error

	switch {
	case err == nil:
		// Deduplicate: bump the occurrence counter and refresh the last
		// timestamp. Always keep the newest technical detail and safe
		// metadata so the operator sees the latest failure context.
		updates := map[string]interface{}{
			"occurrence_count":  gorm.Expr("occurrence_count + 1"),
			"last_occurred_at":  gorm.Expr("CURRENT_TIMESTAMP"),
			"technical_details": alert.TechnicalDetails,
			"safe_metadata":     alert.SafeMetadata,
		}
		if alert.TitleKey != "" {
			updates["title_key"] = alert.TitleKey
		}
		if alert.MessageKey != "" {
			updates["message_key"] = alert.MessageKey
		}
		if alert.Severity != "" {
			updates["severity"] = alert.Severity
		}
		if alert.Category != "" {
			updates["category"] = alert.Category
		}
		return r.db.Model(&existing).Updates(updates).Error

	case errors.Is(err, gorm.ErrRecordNotFound):
		return r.db.Create(alert).Error

	default:
		return err
	}
}

// FindByID returns one alert by UUID.
func (r *SystemAlertRepository) FindByID(id string) (*models.SystemAlert, error) {
	alertID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrSystemAlertNotFound
	}

	var alert models.SystemAlert
	err = r.db.Where("id = ?", alertID).First(&alert).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSystemAlertNotFound
	}
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// List returns system alerts, newest first, optionally filtered by status,
// severity, category and full-text search. limit caps the result set.
func (r *SystemAlertRepository) List(
	status, severity, category, search string,
	limit, offset int32,
) ([]models.SystemAlert, int64, error) {
	var alerts []models.SystemAlert
	var total int64

	query := r.db.Model(&models.SystemAlert{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		query = query.Where(
			"LOWER(alert_code) LIKE ? OR LOWER(category) LIKE ? OR LOWER(source) LIKE ? OR LOWER(message_key) LIKE ? OR LOWER(title_key) LIKE ?",
			like, like, like, like, like,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	if err := query.
		Order("last_occurred_at DESC, created_at DESC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&alerts).Error; err != nil {
		return nil, 0, err
	}

	return alerts, total, nil
}

// AlertStats is the count summary rendered as the alert overview cards.
type AlertStats struct {
	Critical   int64 `json:"critical"`
	Errors     int64 `json:"errors"`
	Warnings   int64 `json:"warnings"`
	Info       int64 `json:"info"`
	Open       int64 `json:"open"`
	Unresolved int64 `json:"unresolved"`
	Total      int64 `json:"total"`
	Security   int64 `json:"security"`
	Resolved   int64 `json:"resolved"`
}

// CountStats returns severity/status count summaries for the alert inbox.
func (r *SystemAlertRepository) CountStats() (*AlertStats, error) {
	stats := &AlertStats{}

	countBySeverity := func(severity string) (int64, error) {
		var count int64
		err := r.db.Model(&models.SystemAlert{}).
			Where("severity = ?", severity).
			Count(&count).Error
		return count, err
	}

	var err error
	if stats.Critical, err = countBySeverity(models.SystemAlertLevelCritical); err != nil {
		return nil, err
	}
	if stats.Errors, err = countBySeverity(models.SystemAlertLevelError); err != nil {
		return nil, err
	}
	if stats.Warnings, err = countBySeverity(models.SystemAlertLevelWarning); err != nil {
		return nil, err
	}
	if stats.Info, err = countBySeverity(models.SystemAlertLevelInfo); err != nil {
		return nil, err
	}

	if err := r.db.Model(&models.SystemAlert{}).
		Where("status = ?", models.SystemAlertStatusOpen).
		Count(&stats.Open).Error; err != nil {
		return nil, err
	}
	stats.Unresolved = stats.Open

	if err := r.db.Model(&models.SystemAlert{}).
		Where("status = ?", models.SystemAlertStatusResolved).
		Count(&stats.Resolved).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&models.SystemAlert{}).
		Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// Security category: alerts raised by the auth/security layer.
	if err := r.db.Model(&models.SystemAlert{}).
		Where("category = ?", "security").
		Count(&stats.Security).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// CountOpen returns the number of currently open alerts.
func (r *SystemAlertRepository) CountOpen() (int64, error) {
	var count int64
	err := r.db.Model(&models.SystemAlert{}).
		Where("status = ?", models.SystemAlertStatusOpen).
		Count(&count).Error
	return count, err
}

// MarkResolved marks an open alert resolved by the given operator id.
func (r *SystemAlertRepository) MarkResolved(
	id string,
	resolvedBy uuid.UUID,
) error {
	alertID, err := uuid.Parse(id)
	if err != nil {
		return ErrSystemAlertNotFound
	}

	result := r.db.Model(&models.SystemAlert{}).
		Where("id = ? AND status = ?", alertID, models.SystemAlertStatusOpen).
		Updates(map[string]interface{}{
			"status":      models.SystemAlertStatusResolved,
			"resolved_by": resolvedBy,
			"resolved_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSystemAlertNotFound
	}
	return nil
}

// MarkRead marks an alert as read (without resolving it).
func (r *SystemAlertRepository) MarkRead(id string) error {
	alertID, err := uuid.Parse(id)
	if err != nil {
		return ErrSystemAlertNotFound
	}

	result := r.db.Model(&models.SystemAlert{}).
		Where("id = ?", alertID).
		Updates(map[string]interface{}{
			"read_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSystemAlertNotFound
	}
	return nil
}
