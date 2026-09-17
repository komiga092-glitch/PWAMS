package services_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type fakeAlertRepo struct {
	alert       *models.SystemAlert
	createCalls int
}

func (f *fakeAlertRepo) CreateOrIncrement(alert *models.SystemAlert) error {
	f.createCalls++
	// Simulate GORM's BeforeCreate hook, which the real repository triggers
	// via db.Create(). This ensures the Level column is synchronised with
	// Severity just as it would be in production.
	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	if alert.Severity == "" {
		alert.Severity = models.SystemAlertLevelError
	}
	alert.Level = alert.Severity
	copied := *alert
	f.alert = &copied
	return nil
}

func (f *fakeAlertRepo) Create(alert *models.SystemAlert) error {
	f.createCalls++
	// Same hook simulation as CreateOrIncrement.
	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	if alert.Severity == "" {
		alert.Severity = models.SystemAlertLevelError
	}
	alert.Level = alert.Severity
	copied := *alert
	f.alert = &copied
	return nil
}

func (f *fakeAlertRepo) FindByID(id string) (*models.SystemAlert, error) {
	return nil, nil
}

func (f *fakeAlertRepo) List(_, _, _, _ string, _ int32, _ int32) ([]models.SystemAlert, int64, error) {
	return nil, 0, nil
}

func (f *fakeAlertRepo) CountStats() (*repository.AlertStats, error) { return nil, nil }
func (f *fakeAlertRepo) CountOpen() (int64, error)                   { return 0, nil }
func (f *fakeAlertRepo) MarkResolved(_ string, _ uuid.UUID) error {
	return nil
}
func (f *fakeAlertRepo) MarkRead(_ string) error { return nil }

func TestCreateAlert_AllSeverities(t *testing.T) {
	cases := []struct {
		name     string
		severity string
	}{
		{"info", models.SystemAlertLevelInfo},
		{"warning", models.SystemAlertLevelWarning},
		{"error", models.SystemAlertLevelError},
		{"critical", models.SystemAlertLevelCritical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAlertRepo{}
			svc := services.NewSystemAlertService(repo)
			err := svc.Create(services.AlertOptions{
				Code:     "ALERT_TEST",
				Severity: tc.severity,
				Source:   "test",
			})
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}
			if repo.alert == nil {
				t.Fatal("no alert was persisted")
			}
			if repo.alert.Severity != tc.severity {
				t.Errorf("Severity = %q, want %q", repo.alert.Severity, tc.severity)
			}
			if repo.alert.Level != tc.severity {
				t.Errorf("Level = %q, want %q", repo.alert.Level, tc.severity)
			}
		})
	}
}

func TestCreateAlert_UnexpectedPanicIsCritical(t *testing.T) {
	repo := &fakeAlertRepo{}
	svc := services.NewSystemAlertService(repo)
	if err := svc.Fire("ALERT_UNEXPECTED_PANIC", "http", "", "stack trace"); err != nil {
		t.Fatalf("Fire(ALERT_UNEXPECTED_PANIC) returned error: %v", err)
	}
	if repo.alert == nil {
		t.Fatal("no alert was persisted for ALERT_UNEXPECTED_PANIC")
	}
	if repo.alert.Severity != models.SystemAlertLevelCritical {
		t.Errorf("Severity = %q, want %q", repo.alert.Severity, models.SystemAlertLevelCritical)
	}
	if repo.alert.Level != models.SystemAlertLevelCritical {
		t.Errorf("Level = %q, want %q", repo.alert.Level, models.SystemAlertLevelCritical)
	}
}

func TestCreateAlert_EmptySeverityDefaultsToError(t *testing.T) {
	repo := &fakeAlertRepo{}
	svc := services.NewSystemAlertService(repo)
	if err := svc.Create(services.AlertOptions{Code: "ALERT_TEST", Source: "test"}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.alert.Severity != models.SystemAlertLevelError {
		t.Errorf("Severity = %q, want %q", repo.alert.Severity, models.SystemAlertLevelError)
	}
	if repo.alert.Level != models.SystemAlertLevelError {
		t.Errorf("Level = %q, want %q", repo.alert.Level, models.SystemAlertLevelError)
	}
}

func TestCreateAlert_InvalidSeverityFallsBackToError(t *testing.T) {
	repo := &fakeAlertRepo{}
	svc := services.NewSystemAlertService(repo)
	if err := svc.Create(services.AlertOptions{Code: "ALERT_TEST", Severity: "bogus", Source: "test"}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.alert.Severity != models.SystemAlertLevelError {
		t.Errorf("Severity = %q, want %q", repo.alert.Severity, models.SystemAlertLevelError)
	}
	if repo.alert.Level != models.SystemAlertLevelError {
		t.Errorf("Level = %q, want %q", repo.alert.Level, models.SystemAlertLevelError)
	}
}

func TestSystemAlertModel_BeforeCreateSyncsLevel(t *testing.T) {
	alert := &models.SystemAlert{
		AlertCode: "ALERT_TEST",
		Severity:  models.SystemAlertLevelCritical,
		Source:    "test",
	}
	if err := alert.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if alert.Level != models.SystemAlertLevelCritical {
		t.Errorf("Level = %q, want %q", alert.Level, models.SystemAlertLevelCritical)
	}
}

func TestSystemAlertModel_BeforeCreateDefaultSeverity(t *testing.T) {
	alert := &models.SystemAlert{AlertCode: "ALERT_TEST"}
	if err := alert.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if alert.Severity != models.SystemAlertLevelError {
		t.Errorf("Severity = %q, want %q", alert.Severity, models.SystemAlertLevelError)
	}
	if alert.Level != models.SystemAlertLevelError {
		t.Errorf("Level = %q, want %q", alert.Level, models.SystemAlertLevelError)
	}
}

func TestSystemAlertModel_BeforeSaveSyncsLevel(t *testing.T) {
	alert := &models.SystemAlert{
		AlertCode: "ALERT_TEST",
		Severity:  models.SystemAlertLevelWarning,
		Level:     "stale",
	}
	if err := alert.BeforeSave(nil); err != nil {
		t.Fatalf("BeforeSave error: %v", err)
	}
	if alert.Level != models.SystemAlertLevelWarning {
		t.Errorf("Level = %q, want %q", alert.Level, models.SystemAlertLevelWarning)
	}
}

func TestSystemAlertModel_LevelNeverEmptyAfterHooks(t *testing.T) {
	severities := []string{
		models.SystemAlertLevelInfo,
		models.SystemAlertLevelWarning,
		models.SystemAlertLevelError,
		models.SystemAlertLevelCritical,
		"",
		"bogus",
	}
	for _, sev := range severities {
		alert := &models.SystemAlert{AlertCode: "ALERT_TEST", Severity: sev}
		if err := alert.BeforeCreate(nil); err != nil {
			t.Fatalf("BeforeCreate(%q) error: %v", sev, err)
		}
		if strings.TrimSpace(alert.Level) == "" {
			t.Errorf("Level is empty after BeforeCreate with Severity=%q", sev)
		}
	}
}

func TestCreateAlert_PersistsPresentation(t *testing.T) {
	// Canonical path: Source drives the legacy presentation column so the
	// NOT NULL system_alerts.presentation constraint is always satisfied.
	repo := &fakeAlertRepo{}
	svc := services.NewSystemAlertService(repo)
	if err := svc.Create(services.AlertOptions{Code: "ALERT_TEST", Severity: "error", Source: "sync"}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.alert.Source != "sync" || repo.alert.Presentation != "sync" {
		t.Errorf("Source=%q Presentation=%q, want sync/sync", repo.alert.Source, repo.alert.Presentation)
	}

	// Legacy path: Presentation (no Source) still populates both fields.
	repo2 := &fakeAlertRepo{}
	svc2 := services.NewSystemAlertService(repo2)
	if err := svc2.Create(services.AlertOptions{Code: "ALERT_TEST", Severity: "error", Presentation: "http"}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo2.alert.Source != "http" || repo2.alert.Presentation != "http" {
		t.Errorf("Source=%q Presentation=%q, want http/http", repo2.alert.Source, repo2.alert.Presentation)
	}
}

// TestCreateAlert_AllRequiredDatabaseFieldsPopulated verifies the service
// populates EVERY NOT NULL column of system_alerts (audited against the live
// schema in the project report) so no next-NULL-constraint whack-a-mole is
// possible: level, message_key, presentation, status, created_at,
// alert_code, category, severity, title_key, source, occurrence_count.
func TestCreateAlert_AllRequiredDatabaseFieldsPopulated(t *testing.T) {
	repo := &fakeAlertRepo{}
	svc := services.NewSystemAlertService(repo)
	if err := svc.Create(services.AlertOptions{
		Code:             "ALERT_UNEXPECTED_PANIC",
		Source:           "http",
		TechnicalDetails: "stack",
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	a := repo.alert
	if a == nil {
		t.Fatal("no alert was persisted")
	}
	if strings.TrimSpace(a.AlertCode) == "" {
		t.Error("AlertCode empty (alert_code is NOT NULL)")
	}
	if strings.TrimSpace(a.Category) == "" {
		t.Error("Category empty (category is NOT NULL)")
	}
	if strings.TrimSpace(a.Severity) == "" {
		t.Error("Severity empty (severity is NOT NULL)")
	}
	if strings.TrimSpace(a.Level) == "" {
		t.Error("Level empty (level is NOT NULL)")
	}
	if strings.TrimSpace(a.TitleKey) == "" {
		t.Error("TitleKey empty (title_key is NOT NULL)")
	}
	if strings.TrimSpace(a.MessageKey) == "" {
		t.Error("MessageKey empty (message_key is NOT NULL)")
	}
	if strings.TrimSpace(a.Source) == "" {
		t.Error("Source empty (source is NOT NULL)")
	}
	if strings.TrimSpace(a.Presentation) == "" {
		t.Error("Presentation empty — system_alerts.presentation is NOT NULL and every alert must supply a channel")
	}
	if a.Status != models.SystemAlertStatusOpen {
		t.Errorf("Status = %q, want open (status is NOT NULL)", a.Status)
	}
	if a.OccurrenceCount < 1 {
		t.Errorf("OccurrenceCount = %d, want >= 1 (occurrence_count is NOT NULL)", a.OccurrenceCount)
	}
	if a.FirstOccurredAt == nil || a.LastOccurredAt == nil {
		t.Error("occurrence timestamps missing (first/last_occurred_at are NOT NULL for new rows)")
	}
	if a.CreatedAt.IsZero() {
		t.Error("CreatedAt zero — created_at is NOT NULL and must be populated on insert")
	}
}

// TestCreateAlert_RepoFailureReturnsErrorWithoutPanic proves that a failed
// alert insert (the presentation NULL bug) cannot crash the caller: the
// service returns a wrapped, non-fatal error (best-effort alerting).
func TestCreateAlert_RepoFailureReturnsErrorWithoutPanic(t *testing.T) {
	svc := services.NewSystemAlertService(failAlertRepo{})
	err := svc.Create(services.AlertOptions{Code: "ALERT_TEST", Source: "test"})
	if err == nil {
		t.Fatal("expected an error when the repository insert fails")
	}
	if !errors.Is(err, services.ErrSystemAlertCreateFailed) {
		t.Errorf("expected ErrSystemAlertCreateFailed, got %v", err)
	}
}

func TestSystemAlertModel_PresentationSyncsFromSource(t *testing.T) {
	alert := &models.SystemAlert{
		AlertCode: "ALERT_TEST",
		Severity:  models.SystemAlertLevelCritical,
		Source:    "http",
	}
	if err := alert.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if alert.Presentation != "http" {
		t.Errorf("Presentation = %q, want http", alert.Presentation)
	}
}

func TestSystemAlertModel_PresentationDefaultsToSystem(t *testing.T) {
	alert := &models.SystemAlert{AlertCode: "ALERT_TEST"}
	if err := alert.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate error: %v", err)
	}
	if alert.Presentation != "system" {
		t.Errorf("Presentation = %q, want system (fallback for the NOT NULL column)", alert.Presentation)
	}
}

// failAlertRepo simulates a database insert constraint failure (the real
// world presentation NULL bug) — the repository returns an error.
type failAlertRepo struct{}

func (failAlertRepo) CreateOrIncrement(*models.SystemAlert) error {
	return errors.New("simulated constraint violation")
}
func (failAlertRepo) FindByID(string) (*models.SystemAlert, error) { return nil, nil }
func (failAlertRepo) List(_, _, _, _ string, _ int32, _ int32) ([]models.SystemAlert, int64, error) {
	return nil, 0, nil
}
func (failAlertRepo) CountStats() (*repository.AlertStats, error) { return nil, nil }
func (failAlertRepo) CountOpen() (int64, error)                   { return 0, nil }
func (failAlertRepo) MarkResolved(_ string, _ uuid.UUID) error    { return nil }
func (failAlertRepo) MarkRead(_ string) error                     { return nil }

func TestStandardAlertCatalog(t *testing.T) {
	cases := map[string]string{
		"ALERT_UNEXPECTED_PANIC": models.SystemAlertLevelCritical,
		"ALERT_SERVER_ERROR":     models.SystemAlertLevelError,
		"ALERT_DB_CONNECTION":    models.SystemAlertLevelCritical,
		"ALERT_DB_OPERATION":     models.SystemAlertLevelError,
		"ALERT_MIGRATION":        models.SystemAlertLevelCritical,
	}
	for code, want := range cases {
		t.Run(code, func(t *testing.T) {
			def := services.StandardAlert(code)
			if def == nil {
				t.Fatalf("StandardAlert(%q) returned nil", code)
			}
			if def.Severity != want {
				t.Errorf("Severity = %q, want %q", def.Severity, want)
			}
		})
	}
}
