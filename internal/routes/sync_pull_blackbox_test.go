package routes_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/routes"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// ─────────────────────────────────────────────────────────────────────────────
// SYNC-002: a pull page returns only the changes after the supplied cursor.
// The cursor is composite (timestamp + record id): a timestamp alone loses both
// sub-second precision and the tiebreaker between records that share an
// updated_at value, so the next page would replay them.
// ─────────────────────────────────────────────────────────────────────────────

func newSyncTestRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)

	sessionService := services.NewSessionService(repository.NewSessionRepository(db))
	authMiddleware := middleware.NewAuthMiddleware(sessionService)

	router := gin.New()
	routes.RegisterSyncRoutes(
		router,
		handlers.NewSyncHandler(services.NewSyncService(
			repository.NewSyncRepository(db),
			nil,
		), db),
		authMiddleware,
	)
	return router
}

type syncPullResponse struct {
	Success bool   `json:"success"`
	Cursor  string `json:"cursor"`
	HasMore bool   `json:"has_more"`
	Records []struct {
		RecordID string `json:"record_id"`
	} `json:"records"`
}

func TestSyncPullCursorNeverReplaysRecords(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newSyncTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	doPull := func(target string) syncPullResponse {
		rec := reportDo(router, target, token, false)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%.200s", target, rec.Code, rec.Body.String())
		}

		var payload syncPullResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("GET %s: invalid JSON: %v", target, err)
		}

		return payload
	}

	first := doPull("/api/v1/sync/pull?limit=10")
	if !first.Success {
		t.Fatal("first pull did not report success")
	}
	if len(first.Records) == 0 {
		t.Skip("no synchronization records in this database")
	}
	if first.Cursor == "" {
		t.Fatal("first pull returned an empty cursor")
	}
	if !strings.Contains(first.Cursor, "~") {
		t.Errorf("cursor %q carries no id component, so records sharing the "+
			"boundary updated_at cannot be ordered", first.Cursor)
	}

	// The cursor is echoed back verbatim, the way a client that concatenates
	// cursor={cursor} into its next request would: no percent-encoding, so an
	// offset separator that survives only as an encoded character would show
	// up here as a 400 (QA SYNC-002).
	second := doPull("/api/v1/sync/pull?limit=10&cursor=" + first.Cursor)

	seen := make(map[string]bool, len(first.Records))
	for _, record := range first.Records {
		seen[record.RecordID] = true
	}

	for _, record := range second.Records {
		if seen[record.RecordID] {
			t.Errorf("record %s was returned by both pull pages "+
				"(cursor %q replays already-synced rows)",
				record.RecordID, first.Cursor)
		}
	}
}

// TestSyncPullRejectsMalformedCursor keeps the error path explicit: an
// unparsable cursor is a client error, never a silent first page.
func TestSyncPullRejectsMalformedCursor(t *testing.T) {
	db := acquireReportTestDB(t)
	router := newSyncTestRouter(db)

	user := createReportProbeUser(t, db, models.RoleSuperAdmin)
	token := reportSessionToken(t, db, user)

	rec := reportDo(router, "/api/v1/sync/pull?cursor=not-a-cursor", token, false)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed cursor: status=%d, want 400 (body=%.200s)",
			rec.Code, rec.Body.String())
	}
}
