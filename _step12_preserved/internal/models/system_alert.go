package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// System alert severities. Ordering matters: info < warning < error <
// critical. Severity is an *internal* stable identifier used for filtering
// and triage; presentation always happens through i18n keys (alert.severity.*).
const (
	SystemAlertLevelInfo     = "info"
	SystemAlertLevelWarning  = "warning"
	SystemAlertLevelError    = "error"
	SystemAlertLevelCritical = "critical"
)

// AlertSeverities is the canonical, ordered list of supported severities.
var AlertSeverities = []string{
	SystemAlertLevelInfo,
	SystemAlertLevelWarning,
	SystemAlertLevelError,
	SystemAlertLevelCritical,
}

// ErrInvalidAlertSeverity reports an unrecognised severity identifier.
var ErrInvalidAlertSeverity = &AlertSeverityError{}

// AlertSeverityError is a typed error so callers can distinguish an invalid
// severity from other failures.
type AlertSeverityError struct{}

func (e *AlertSeverityError) Error() string {
	return "unknown system alert severity"
}

// ValidAlertSeverity reports whether level is one of the supported severities.
func ValidAlertSeverity(level string) bool {
	_, err := NormalizeAlertSeverity(level)
	return err == nil
}

// NormalizeAlertSeverity returns the canonical severity for the given value,
// or an error for unrecognised values. Empty maps to error (the default).
func NormalizeAlertSeverity(level string) (string, error) {
	switch level {
	case "", SystemAlertLevelError:
		return SystemAlertLevelError, nil
	case SystemAlertLevelInfo:
		return SystemAlertLevelInfo, nil
	case SystemAlertLevelWarning:
		return SystemAlertLevelWarning, nil
	case SystemAlertLevelCritical:
		return SystemAlertLevelCritical, nil
	default:
		return "", ErrInvalidAlertSeverity
	}
}

// System alert lifecycle statuses.
const (
	SystemAlertStatusOpen     = "open"
	SystemAlertStatusResolved = "resolved"
	SystemAlertStatusRead     = "read"
)

// SystemAlert records a system-level event for privileged operators. Internal
// stable identifiers are never translated: alert_code, category, severity,
// source, message_key and title_key are keys/lookup values used at
// presentation time through the i18n dictionaries. TechnicalDetails holds the
// raw critical technical context (stack traces, failing SQL, request paths)
// and is only ever exposed to actors holding system.alerts.view — the Super
// Admin (and optionally Admin per the permission grant) role. SafeMetadata
// carries structured, non-secret key/value context (e.g. "method", "path")
// that is safe to render in the professional alert detail card.
//
// Field relationship: Severity is the canonical application-facing field
// (mapped to the severity column). Level is the legacy database column (from
// migration 000004) that is NOT NULL. The BeforeCreate/BeforeSave hooks keep
// the two columns synchronised so the legacy constraint is always satisfied.
type SystemAlert struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AlertCode string    `gorm:"size:100;not null;default:'';index" json:"alert_code"`
	Category  string    `gorm:"size:50;not null;default:'';index" json:"category"`
	Severity  string    `gorm:"size:20;not null;default:error;index" json:"severity"`
	// Level is the legacy database column (migration 000004). It is kept in
	// sync with Severity by the BeforeCreate/BeforeSave hooks so the NOT NULL
	// constraint on the legacy column is always satisfied. Application code
	// should use Severity; Level is a DB-only mirror.
	Level string `gorm:"size:20;not null;default:error;index" json:"level"`
	// Presentation is the legacy database column (migration 000004) that
	// migration 000005 superseded with Source. It identifies the channel /
	// presentation context in which the alert fired (e.g. "http", "sync",
	// "email"). The column is NOT NULL in the schema, so the model mirrors it
	// and keeps it in sync with Source so every insert supplies a value.
	// Application code should use Source; Presentation is a DB-only mirror.
	Presentation     string     `gorm:"size:100;not null;default:'';index" json:"presentation,omitempty"`
	TitleKey         string     `gorm:"size:200;not null;default:''" json:"title_key"`
	MessageKey       string     `gorm:"size:200;not null" json:"message_key"`
	SafeMetadata     string     `gorm:"type:text" json:"safe_metadata,omitempty"`
	TechnicalDetails *string    `gorm:"type:text" json:"technical_details,omitempty"`
	Source           string     `gorm:"size:100;not null;default:'';index" json:"source"`
	OccurrenceCount  int        `gorm:"not null;default:1" json:"occurrence_count"`
	FirstOccurredAt  *time.Time `json:"first_occurred_at,omitempty"`
	LastOccurredAt   *time.Time `json:"last_occurred_at,omitempty"`
	Status           string     `gorm:"size:20;not null;default:open;index" json:"status"`
	ReadAt           *time.Time `json:"read_at,omitempty"`
	ResolvedBy       *uuid.UUID `gorm:"type:uuid" json:"resolved_by,omitempty"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	CreatedAt        time.Time  `gorm:"not null;index" json:"created_at"`
}

// syncAlertLevel ensures the legacy level column mirrors the canonical
// Severity field, and the legacy presentation column mirrors Source. Called
// from both BeforeCreate and BeforeSave so the values are correct for INSERT
// and UPDATE operations — the presentation column is NOT NULL so it must
// always be populated.
func syncAlertLevel(alert *SystemAlert) {
	if alert.Severity == "" {
		alert.Severity = SystemAlertLevelError
	}
	alert.Level = alert.Severity
	if alert.Presentation == "" {
		if alert.Source != "" {
			alert.Presentation = alert.Source
		} else {
			alert.Presentation = "system"
		}
	}
}

func (alert *SystemAlert) BeforeCreate(_ *gorm.DB) error {
	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	syncAlertLevel(alert)
	if alert.OccurrenceCount == 0 {
		alert.OccurrenceCount = 1
	}
	now := time.Now().UTC()
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = now
	}
	if alert.FirstOccurredAt == nil {
		alert.FirstOccurredAt = &now
	}
	if alert.LastOccurredAt == nil {
		alert.LastOccurredAt = &now
	}
	return nil
}

// BeforeSave keeps the legacy level column synchronised with Severity on
// every save (including updates via CreateOrIncrement in the repository).
func (alert *SystemAlert) BeforeSave(_ *gorm.DB) error {
	syncAlertLevel(alert)
	return nil
}
