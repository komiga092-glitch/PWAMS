package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// alertDedupTTL is how long an identical alert (same code presented in the
// same way) is suppressed after being recorded. This prevents a failing
// endpoint from flooding the alert inbox with thousands of identical rows.
const alertDedupTTL = 5 * time.Minute

// alertMonitor wraps the alert service so finite/infinite states remain
// simple: a nil service makes recording a no-op without a nil-check at every
// call site. The instance is created per middleware mount.
type alertMonitor struct {
	service *services.SystemAlertService
}

// record converts a panic or server-error incident into a system alert.
// Panics are always CRITICAL (unhandled panics indicate a serious defect);
// HTTP 5xx responses are recorded as ERROR alerts. Both are best-effort.
func (m alertMonitor) record(level, code, messageKey, presentation, details string) {
	_ = m.service.Create(services.AlertOptions{
		Code:             code,
		Severity:         level,
		MessageKey:       messageKey,
		Source:           presentation,
		Presentation:     presentation,
		TechnicalDetails: details,
	})
}

// SystemAlertMonitor converts panics and server-side (>= 500) responses into
// system alerts for privileged operators. The alert payload includes
// technical detail (panic stack traces, request method+path, error message)
// which is stored for the privileged audience only — it is never rendered to
// ordinary users.
//
// The middleware is best-effort: if the alert service is nil, it still
// recovers panics (returning a 500) and silently skips alert recording. It
// never blocks or mutates the request flow beyond recovering a panic.
func SystemAlertMonitor(alertService *services.SystemAlertService) gin.HandlerFunc {
	var (
		dedupMu sync.Mutex
		dedup   = make(map[string]time.Time)
	)

	monitor := alertMonitor{service: alertService}

	purge := func() {
		now := time.Now()
		for key, at := range dedup {
			if now.Sub(at) > alertDedupTTL {
				delete(dedup, key)
			}
		}
	}

	record := func(level, code, messageKey, presentation, details string) {
		if alertService == nil {
			return
		}
		dedupMu.Lock()
		purge()
		key := AlertDedupKey(code, presentation, details)
		if _, seen := dedup[key]; seen {
			dedupMu.Unlock()
			return
		}
		dedup[key] = time.Now()
		dedupMu.Unlock()

		// Recording an incident must never re-enter the panic recovery that
		// raised it. If the alert service itself panics (pathological
		// infrastructure defect), we recover, log, and keep the safe 5xx —
		// failing to record an alert is never allowed to abort the request
		// with a second, recursive panic.
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("system-alert: alert recording panicked (suppressed): %v", rec)
				}
			}()
			monitor.record(level, code, messageKey, presentation, details)
		}()
	}

	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				details := fmt.Sprintf(
					"panic: %v\nmethod=%s path=%s\n%s",
					rec,
					c.Request.Method,
					c.Request.URL.RequestURI(),
					debug.Stack(),
				)
				record(
					models.SystemAlertLevelCritical,
					"ALERT_UNEXPECTED_PANIC",
					"alert.unhandled_panic",
					"http",
					details,
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		c.Next()

		status := c.Writer.Status()
		if status >= http.StatusInternalServerError {
			details := fmt.Sprintf(
				"method=%s path=%s client=%s",
				c.Request.Method,
				c.Request.URL.RequestURI(),
				c.ClientIP(),
			)
			record(
				models.SystemAlertLevelError,
				"ALERT_SERVER_ERROR",
				statusMessageKey(status),
				"http",
				details,
			)
		}
	}
}

// statusMessageKey maps an HTTP status code to a stable i18n alert key.
func statusMessageKey(status int) string {
	switch status {
	case http.StatusNotFound:
		return "alert.http_404"
	case http.StatusInternalServerError:
		return "alert.http_500"
	case http.StatusBadGateway:
		return "alert.http_502"
	case http.StatusServiceUnavailable:
		return "alert.http_503"
	default:
		return fmt.Sprintf("alert.http_%d", status)
	}
}

// AlertDedupKey builds a stable, bounded-size deduplication key for an
// incident. Technical details (panic stack traces) can be large and differ in
// address bytes between runs, so the raw payload is never kept in the dedup
// map; a SHA-256 digest of code|presentation|details keeps the map small and
// still collapses genuinely identical incidents.
func AlertDedupKey(code, presentation, details string) string {
	digest := sha256.Sum256([]byte(
		strings.ToLower(code) + "|" +
			strings.ToLower(presentation) + "|" +
			details,
	))
	return hex.EncodeToString(digest[:])
}
