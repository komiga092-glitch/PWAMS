package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

// ErrSystemAlertCreateFailed is returned when a system alert could not be
// persisted (never fatal to the caller — alerting is best effort, with a log
// fallback).
var ErrSystemAlertCreateFailed = errors.New("system alert create failed")

// Stable alert categories (internal identifiers; only presentation is
// translated).
const (
	AlertCategoryDatabase   = "database"
	AlertCategoryStorage    = "storage"
	AlertCategorySync       = "sync"
	AlertCategoryEmail      = "email"
	AlertCategoryBackground = "background"
	AlertCategorySecurity   = "security"
	AlertCategoryHTTP       = "http"
	AlertCategoryConfig     = "config"
	AlertCategoryMigration  = "migration"
	AlertCategoryExternal   = "external"
)

// StandardSystemAlert defines the stable contract for one system failure
// class: code, category, default severity, and the i18n keys used for the
// alert title and message. Codes and categories are internal identifiers that
// are never translated; only the human-readable title/message keys are.
type StandardSystemAlert struct {
	Code       string
	Category   string
	Severity   string
	TitleKey   string
	MessageKey string
}

// AlertOptions carries the payload for creating a system alert. Internal
// identifiers (Code, Category, Severity, Source) are stable; human-facing
// text always goes through TitleKey / MessageKey i18n keys. SafeMetadata is
// a JSON-encoded, non-secret key/value string (e.g. {"method":"POST"}).
type AlertOptions struct {
	Code             string
	Category         string
	Severity         string
	TitleKey         string
	MessageKey       string
	Source           string
	SafeMetadata     string
	TechnicalDetails string
	// Presentation is a legacy alias for Source (pre-000005 alert model). It
	// is kept so the middleware and existing callers keep compiling; Source
	// wins when both are set.
	Presentation string
}

// SystemAlertRepository is the persistence contract the service depends on.
// Defining it as an interface keeps the service unit-testable without a live
// database. The concrete *repository.SystemAlertRepository satisfies it.
type SystemAlertRepository interface {
	CreateOrIncrement(alert *models.SystemAlert) error
	FindByID(id string) (*models.SystemAlert, error)
	List(status, severity, category, search string, limit, offset int32) ([]models.SystemAlert, int64, error)
	CountStats() (*repository.AlertStats, error)
	CountOpen() (int64, error)
	MarkResolved(id string, resolvedBy uuid.UUID) error
	MarkRead(id string) error
}

// SystemAlertService records and queries system-level alerts for privileged
// operators (Super Admin by default). Creating an alert never fails the
// operation that triggered it: the service logs the failure instead.
type SystemAlertService struct {
	alertRepo SystemAlertRepository
}

func NewSystemAlertService(alertRepo SystemAlertRepository) *SystemAlertService {
	return &SystemAlertService{alertRepo: alertRepo}
}

// standardSystemAlerts is the canonical alert catalog. New failure classes
// must be added here so the Super Admin inbox uses stable, deduplicatable
// identifiers instead of ad-hoc strings.
var standardSystemAlerts = map[string]StandardSystemAlert{
	"ALERT_DB_CONNECTION":       {Code: "ALERT_DB_CONNECTION", Category: AlertCategoryDatabase, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.db_connection", MessageKey: "alert.db_connection"},
	"ALERT_DB_OPERATION":        {Code: "ALERT_DB_OPERATION", Category: AlertCategoryDatabase, Severity: models.SystemAlertLevelError, TitleKey: "alert.title.db_operation", MessageKey: "alert.db_operation"},
	"ALERT_MIGRATION":           {Code: "ALERT_MIGRATION", Category: AlertCategoryMigration, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.migration", MessageKey: "alert.migration"},
	"ALERT_SYNC_FAILURE":        {Code: "ALERT_SYNC_FAILURE", Category: AlertCategorySync, Severity: models.SystemAlertLevelError, TitleKey: "alert.title.sync_failure", MessageKey: "alert.sync_failure"},
	"ALERT_SYNC_REPEATED":       {Code: "ALERT_SYNC_REPEATED", Category: AlertCategorySync, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.sync_repeated", MessageKey: "alert.sync_repeated"},
	"ALERT_STORAGE":             {Code: "ALERT_STORAGE", Category: AlertCategoryStorage, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.storage", MessageKey: "alert.storage"},
	"ALERT_FILE_UPLOAD":         {Code: "ALERT_FILE_UPLOAD", Category: AlertCategoryStorage, Severity: models.SystemAlertLevelError, TitleKey: "alert.title.file_upload", MessageKey: "alert.file_upload"},
	"ALERT_EMAIL":               {Code: "ALERT_EMAIL", Category: AlertCategoryEmail, Severity: models.SystemAlertLevelWarning, TitleKey: "alert.title.email", MessageKey: "alert.email"},
	"ALERT_BACKGROUND_JOB":      {Code: "ALERT_BACKGROUND_JOB", Category: AlertCategoryBackground, Severity: models.SystemAlertLevelError, TitleKey: "alert.title.background_job", MessageKey: "alert.background_job"},
	"ALERT_AUTH_ANOMALY":        {Code: "ALERT_AUTH_ANOMALY", Category: AlertCategorySecurity, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.auth_anomaly", MessageKey: "alert.auth_anomaly"},
	"ALERT_SECURITY_VIOLATION":  {Code: "ALERT_SECURITY_VIOLATION", Category: AlertCategorySecurity, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.security_violation", MessageKey: "alert.security_violation"},
	"ALERT_UNAUTHORIZED_ACCESS": {Code: "ALERT_UNAUTHORIZED_ACCESS", Category: AlertCategorySecurity, Severity: models.SystemAlertLevelWarning, TitleKey: "alert.title.unauthorized_access", MessageKey: "alert.unauthorized_access"},
	"ALERT_UNEXPECTED_PANIC":    {Code: "ALERT_UNEXPECTED_PANIC", Category: AlertCategoryHTTP, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.unexpected_panic", MessageKey: "alert.unhandled_panic"},
	"ALERT_SERVER_ERROR":        {Code: "ALERT_SERVER_ERROR", Category: AlertCategoryHTTP, Severity: models.SystemAlertLevelError, TitleKey: "alert.title.server_error", MessageKey: "alert.http_500"},
	"ALERT_EXTERNAL_SERVICE":    {Code: "ALERT_EXTERNAL_SERVICE", Category: AlertCategoryExternal, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.external_service", MessageKey: "alert.external_service"},
	"ALERT_CONFIGURATION":       {Code: "ALERT_CONFIGURATION", Category: AlertCategoryConfig, Severity: models.SystemAlertLevelCritical, TitleKey: "alert.title.configuration", MessageKey: "alert.configuration"},
}

// Create records a new alert (or increments an existing open one by code +
// source). When opts.Code matches the standard alert catalog, the category,
// severity, title and message keys are derived from the catalog entry and any
// explicitly-set fields win. A caller-supplied Severity is honoured verbatim;
// when the caller leaves Severity empty, the catalog entry's severity is
// authoritative (so catalogued failures such as ALERT_UNEXPECTED_PANIC always
// receive their intended CRITICAL severity). Legacy callers that only pass
// Presentation + MessageKey + Presentation keep working.
func (s *SystemAlertService) Create(opts AlertOptions) error {
	code := strings.ToUpper(strings.TrimSpace(opts.Code))
	category := strings.TrimSpace(opts.Category)
	explicitSeverity := strings.TrimSpace(opts.Severity)
	severity, _ := models.NormalizeAlertSeverity(explicitSeverity)
	titleKey := strings.TrimSpace(opts.TitleKey)
	messageKey := strings.TrimSpace(opts.MessageKey)
	source := strings.TrimSpace(opts.Source)
	presentation := strings.TrimSpace(opts.Presentation)
	if source == "" {
		source = presentation
	}
	if presentation == "" {
		presentation = source
	}
	// The legacy presentation column is NOT NULL, so it always needs a
	// concrete value. When neither Source nor Presentation was supplied,
	// "system" is the established fallback (see migrate.go backfill).
	if source == "" {
		source = "system"
	}
	if presentation == "" {
		presentation = "system"
	}

	// Resolve the canonical definition when a catalog code was supplied.
	if def := StandardAlert(code); def != nil {
		if category == "" {
			category = def.Category
		}
		// The catalog severity is authoritative unless the caller explicitly
		// supplied a different severity. Without this, an empty caller
		// severity would be pre-normalised to "error" and the catalogued
		// CRITICAL default (ALERT_UNEXPECTED_PANIC, ALERT_DB_CONNECTION, ...)
		// would never be applied.
		if explicitSeverity == "" || !models.ValidAlertSeverity(explicitSeverity) {
			severity = def.Severity
		}
		if titleKey == "" {
			titleKey = def.TitleKey
		}
		if messageKey == "" {
			messageKey = def.MessageKey
		}
	}

	if messageKey == "" {
		messageKey = "alert.generic_failure"
	}
	if titleKey == "" {
		titleKey = "alert.title.generic_failure"
	}
	if !models.ValidAlertSeverity(severity) {
		severity = models.SystemAlertLevelError
	}

	now := time.Now().UTC()
	alert := &models.SystemAlert{
		AlertCode:        code,
		Category:         category,
		Severity:         severity,
		TitleKey:         titleKey,
		MessageKey:       messageKey,
		SafeMetadata:     strings.TrimSpace(opts.SafeMetadata),
		TechnicalDetails: nil,
		Source:           source,
		Presentation:     presentation,
		OccurrenceCount:  1,
		FirstOccurredAt:  &now,
		LastOccurredAt:   &now,
		CreatedAt:        now,
		Status:           models.SystemAlertStatusOpen,
	}
	if details := strings.TrimSpace(opts.TechnicalDetails); details != "" {
		agent := details
		alert.TechnicalDetails = &agent
	}

	if err := s.alertRepo.CreateOrIncrement(alert); err != nil {
		fmt.Printf("system-alert: failed to record alert %q: %v\n", code, err)
		return fmt.Errorf("%w: %v", ErrSystemAlertCreateFailed, err)
	}
	return nil
}

// Fire is a convenience wrapper for Create that uses the standard alert
// catalog entry for the given code and attaches a safe metadata payload.
func (s *SystemAlertService) Fire(code, source, safeMetadata, technicalDetails string) error {
	return s.Create(AlertOptions{
		Code:             code,
		Source:           source,
		SafeMetadata:     safeMetadata,
		TechnicalDetails: technicalDetails,
	})
}

// List returns alerts (newest first) with optional status/severity/category
// and search filters. A nil limit caps the result set at 100 records.
func (s *SystemAlertService) List(status, severity, category, search string, limit, offset int32) ([]models.SystemAlert, int64, error) {
	status = strings.TrimSpace(status)
	severity = strings.ToLower(strings.TrimSpace(severity))
	category = strings.ToLower(strings.TrimSpace(category))
	search = strings.TrimSpace(search)
	return s.alertRepo.List(status, severity, category, search, limit, offset)
}

// Stats returns the summary counts for the alert overview cards.
func (s *SystemAlertService) Stats() (*repository.AlertStats, error) {
	return s.alertRepo.CountStats()
}

// Resolve marks an open alert as resolved. Only the given operator (who must
// be the authenticated privileged actor) is recorded as the resolver.
func (s *SystemAlertService) Resolve(id string, resolvedBy uuid.UUID) error {
	return s.alertRepo.MarkResolved(id, resolvedBy)
}

// ByID returns a single alert by ID (used by the alert detail card).
func (s *SystemAlertService) ByID(id string) (*models.SystemAlert, error) {
	return s.alertRepo.FindByID(id)
}

// MarkRead marks an alert as read without resolving it.
func (s *SystemAlertService) MarkRead(id string) error {
	return s.alertRepo.MarkRead(id)
}

// CountOpen returns the number of currently open alerts.
func (s *SystemAlertService) CountOpen() (int64, error) {
	return s.alertRepo.CountOpen()
}

// StandardAlert returns the canonical definition for a code, or nil when the
// code is not catalogued. Uncatalogued codes still produce a generic alert.
func StandardAlert(code string) *StandardSystemAlert {
	def, ok := standardSystemAlerts[strings.TrimSpace(code)]
	if !ok {
		return nil
	}
	return &def
}
