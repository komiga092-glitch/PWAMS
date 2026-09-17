package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// SyncPullMaxLimit caps sync pull page sizes so a single request can
// never stream an unbounded result set.
const SyncPullMaxLimit = 500

// syncCursorSeparator joins the composite cursor components.
const syncCursorSeparator = "~"

// SyncCursor is the composite keyset pagination cursor for sync pulls.
// Time alone is not enough because many records can share the same
// updated_at value; ID breaks those ties deterministically.
type SyncCursor struct {
	Time time.Time
	ID   uuid.UUID
}

// IsZero reports whether the cursor points at the very first page.
func (c SyncCursor) IsZero() bool {
	return c.Time.IsZero() && c.ID == uuid.Nil
}

// EncodeSyncCursor renders the composite wire format
// "<RFC3339Nano>~<uuid>". Zero cursors encode as an empty string.
func EncodeSyncCursor(cursor SyncCursor) string {
	if cursor.IsZero() {
		return ""
	}

	return cursor.Time.Format(time.RFC3339Nano) +
		syncCursorSeparator +
		cursor.ID.String()
}

// ParseSyncCursor decodes the wire format produced by
// EncodeSyncCursor. Whitespace-only input selects the first page.
// Legacy time-only cursors (no ID component) remain parseable and map
// to uuid.Nil.
func ParseSyncCursor(raw string) (SyncCursor, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return SyncCursor{}, nil
	}

	parts := strings.Split(trimmed, syncCursorSeparator)

	switch len(parts) {
	case 1:
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return SyncCursor{}, fmt.Errorf(
				"invalid sync cursor timestamp: %w",
				err,
			)
		}

		return SyncCursor{Time: t, ID: uuid.Nil}, nil

	case 2:
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return SyncCursor{}, fmt.Errorf(
				"invalid sync cursor timestamp: %w",
				err,
			)
		}

		id, err := uuid.Parse(parts[1])
		if err != nil {
			return SyncCursor{}, fmt.Errorf(
				"invalid sync cursor id: %w",
				err,
			)
		}

		return SyncCursor{Time: t, ID: id}, nil

	default:
		return SyncCursor{}, errors.New("invalid sync cursor format")
	}
}

// clampSyncLimit coerces any caller-provided limit into the
// [1, SyncPullMaxLimit] range, defaulting to SyncPullMaxLimit.
func clampSyncLimit(limit int) int {
	if limit <= 0 || limit > SyncPullMaxLimit {
		return SyncPullMaxLimit
	}

	return limit
}

// nextSyncCursor advances the cursor to (updatedAt, id) but never moves
// it backwards, so out-of-order inserts cannot replay old rows.
func nextSyncCursor(
	current SyncCursor,
	updatedAt time.Time,
	id uuid.UUID,
) SyncCursor {
	if updatedAt.After(current.Time) {
		return SyncCursor{Time: updatedAt, ID: id}
	}

	if updatedAt.Equal(current.Time) && id.String() > current.ID.String() {
		return SyncCursor{Time: current.Time, ID: id}
	}

	return current
}

// syncCursorWhere renders the SQL keyset predicate matching rows
// strictly after the cursor. Zero cursors yield an empty predicate.
func syncCursorWhere(cursor SyncCursor) (string, []any) {
	if cursor.IsZero() {
		return "", nil
	}

	return "(updated_at > ? OR (updated_at = ? AND id > ?))",
		[]any{cursor.Time, cursor.Time, cursor.ID}
}

// paginateSyncRecords slices an already-filtered, already-sorted record
// slice into one page and derives the cursor for the next page.
// hasMore reports whether records remain beyond this page.
func paginateSyncRecords(
	records []models.SyncPullRecord,
	limit int,
	cursor SyncCursor,
) (
	[]models.SyncPullRecord,
	SyncCursor,
	bool,
	error,
) {
	pageLimit := clampSyncLimit(limit)

	if len(records) == 0 {
		return []models.SyncPullRecord{}, SyncCursor{}, false, nil
	}

	hasMore := len(records) > pageLimit

	page := records
	if hasMore {
		page = records[:pageLimit]
	}

	result := make([]models.SyncPullRecord, len(page))
	copy(result, page)

	next := nextSyncCursor(
		cursor,
		result[len(result)-1].UpdatedAt,
		result[len(result)-1].RecordID,
	)

	return result, next, hasMore, nil
}
