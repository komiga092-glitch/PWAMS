package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// failingAlertRepo simulates a database insert that violates a constraint
// (exactly what happened for system_alerts.presentation). The monitor must
// still recover the panic and answer a safe 500 — never a second panic.
type failingAlertRepo struct{}

func (failingAlertRepo) CreateOrIncrement(*models.SystemAlert) error {
	return errors.New("simulated NOT NULL violation")
}
func (failingAlertRepo) FindByID(string) (*models.SystemAlert, error) { return nil, nil }
func (failingAlertRepo) List(_, _, _, _ string, _ int32, _ int32) ([]models.SystemAlert, int64, error) {
	return nil, 0, nil
}
func (failingAlertRepo) CountStats() (*repository.AlertStats, error) { return nil, nil }
func (failingAlertRepo) CountOpen() (int64, error)                   { return 0, nil }
func (failingAlertRepo) MarkResolved(_ string, _ uuid.UUID) error    { return nil }
func (failingAlertRepo) MarkRead(_ string) error                     { return nil }

// panickingAlertRepo goes one step further: the alert insert itself panics.
// The monitor's safe-record guard must suppress it and still answer a safe
// 500 — alert recording may never recursively panic.
type panickingAlertRepo struct{}

func (panickingAlertRepo) CreateOrIncrement(*models.SystemAlert) error  { panic("alert repo exploded") }
func (panickingAlertRepo) FindByID(string) (*models.SystemAlert, error) { return nil, nil }
func (panickingAlertRepo) List(_, _, _, _ string, _ int32, _ int32) ([]models.SystemAlert, int64, error) {
	return nil, 0, nil
}
func (panickingAlertRepo) CountStats() (*repository.AlertStats, error) { return nil, nil }
func (panickingAlertRepo) CountOpen() (int64, error)                   { return 0, nil }
func (panickingAlertRepo) MarkResolved(_ string, _ uuid.UUID) error    { return nil }
func (panickingAlertRepo) MarkRead(_ string) error                     { return nil }

func TestSystemAlertMonitor_PanicRecoversTo500_WhenAlertRecordingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := services.NewSystemAlertService(failingAlertRepo{})
	router := gin.New()
	router.Use(middleware.SystemAlertMonitor(svc))
	router.GET("/boom", func(c *gin.Context) {
		panic("handler exploded")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	// A recovered panic must be answered with a safe, empty 500 — the SQL
	// error from the failed alert insert must never leak to ordinary users.
	if body := w.Body.String(); strings.Contains(body, "23502") || strings.Contains(body, "presentation") {
		t.Errorf("response leaked internal error detail: %q", body)
	}
}

func TestSystemAlertMonitor_AlertRecordingPanicIsSuppressed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := services.NewSystemAlertService(panickingAlertRepo{})
	router := gin.New()
	router.Use(middleware.SystemAlertMonitor(svc))
	router.GET("/boom", func(c *gin.Context) {
		panic("handler exploded")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	router.ServeHTTP(w, req)

	// The alert-recording panic must be suppressed inside the monitor; the
	// goroutine must not be torn down and the client must still get a 500.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (alert-recording panic must be suppressed)", w.Code)
	}
}

func TestSystemAlertMonitor_ServerErrorRecordsWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := services.NewSystemAlertService(failingAlertRepo{})
	router := gin.New()
	router.Use(middleware.SystemAlertMonitor(svc))
	// Non-panic 5xx: the monitor records ALERT_SERVER_ERROR (best-effort;
	// a failing repo must not change the response).
	router.GET("/five", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "nope")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/five", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestAlertDedupKey_StableAndBounded(t *testing.T) {
	a := middleware.AlertDedupKey("ALERT_UNEXPECTED_PANIC", "http", "stack trace")
	b := middleware.AlertDedupKey("ALERT_UNEXPECTED_PANIC", "http", "stack trace")
	c2 := middleware.AlertDedupKey("ALERT_UNEXPECTED_PANIC", "http", "different trace")
	if a == "" || a != b {
		t.Error("dedup key must be stable for identical incidents")
	}
	if len(a) > 128 {
		t.Error("dedup key must be bounded regardless of details size")
	}
	if a == c2 {
		t.Error("distinct incidents should produce distinct keys")
	}
}
