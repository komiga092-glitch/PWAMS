package repository

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
)

func TestParseSyncCursor_EmptyIsFirstPage(t *testing.T) {
	cursor, err := ParseSyncCursor("   ")
	if err != nil {
		t.Fatalf("empty cursor must parse: %v", err)
	}
	if !cursor.Time.IsZero() || cursor.ID != uuid.Nil {
		t.Fatalf("empty cursor = %+v, want zero cursor", cursor)
	}
}

func TestParseSyncCursor_CompositeRoundTrip(t *testing.T) {
	in := SyncCursor{
		Time: time.Date(2026, 9, 12, 8, 53, 48, 262648000, time.UTC),
		ID:   uuid.MustParse("0198f4a2-7c1d-7de2-9f3a-5b6c7d8e9f01"),
	}
	encoded := EncodeSyncCursor(in)
	if !strings.Contains(encoded, "~") {
		t.Fatalf("composite cursor %q must contain the separator", encoded)
	}
	out, err := ParseSyncCursor(encoded)
	if err != nil {
		t.Fatalf("composite cursor failed to parse: %v", err)
	}
	if !out.Time.Equal(in.Time) || out.ID != in.ID {
		t.Fatalf("round trip mismatch: in=%+v out=%+v", in, out)
	}
}

func TestParseSyncCursor_LegacyTimeOnly(t *testing.T) {
	legacy := "2026-09-12T08:53:48.262648Z"
	cursor, err := ParseSyncCursor(legacy)
	if err != nil {
		t.Fatalf("legacy time-only cursor must parse: %v", err)
	}
	expected, _ := time.Parse(time.RFC3339, legacy)
	if !cursor.Time.Equal(expected) {
		t.Fatalf("legacy time = %v, want %v", cursor.Time, expected)
	}
	if cursor.ID != uuid.Nil {
		t.Fatalf("legacy cursor id = %s, want uuid.Nil", cursor.ID)
	}
}

func TestParseSyncCursor_Invalid(t *testing.T) {
	invalid := []string{
		"not-a-time",
		"2026-09-12T08:53:48.262648Z~not-a-uuid",
		"2026-13-45T99:99:99Z",
		"2026-09-12T08:53:48.262648Z~",
		"~~",
		"2026-09-12T08:53:48Z~00000000-0000-0000-0000-00000000000g",
	}
	for _, value := range invalid {
		if _, err := ParseSyncCursor(value); err == nil {
			t.Fatalf("cursor %q must be rejected", value)
		}
	}
}

func TestClampSyncLimit(t *testing.T) {
	cases := map[int]int{
		-5:      SyncPullMaxLimit,
		0:       SyncPullMaxLimit,
		1:       1,
		499:     499,
		500:     SyncPullMaxLimit,
		501:     SyncPullMaxLimit,
		999999:  SyncPullMaxLimit,
		1000000: SyncPullMaxLimit,
	}
	for in, want := range cases {
		if got := clampSyncLimit(in); got != want {
			t.Fatalf("clampSyncLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

// filterBySyncCursor simulates the SQL keyset predicate
// (updated_at > cursor.Time OR (updated_at = cursor.Time AND id > cursor.ID)).
func filterBySyncCursor(records []models.SyncPullRecord, cursor SyncCursor) []models.SyncPullRecord {
	var filtered []models.SyncPullRecord
	for _, r := range records {
		if r.UpdatedAt.After(cursor.Time) {
			filtered = append(filtered, r)
			continue
		}
		if r.UpdatedAt.Equal(cursor.Time) && r.RecordID.String() > cursor.ID.String() {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func TestPaginateSyncRecords_600IdenticalTimestamps(t *testing.T) {
	const total = 600
	const limit = 500
	now := time.Date(2026, 9, 12, 8, 53, 48, 0, time.UTC)

	records := make([]models.SyncPullRecord, total)
	for i := 0; i < total; i++ {
		records[i] = models.SyncPullRecord{
			EntityType: "persons",
			RecordID:   uuid.New(),
			Version:    1,
			UpdatedAt:  now,
			IsDeleted:  false,
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].RecordID.String() < records[j].RecordID.String()
	})

	page1, cursor1, hasMore1, err := paginateSyncRecords(records, limit, SyncCursor{})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if !hasMore1 {
		t.Fatal("page 1: expected hasMore=true")
	}
	if len(page1) != limit {
		t.Fatalf("page 1: got %d records, want %d", len(page1), limit)
	}

	filtered := filterBySyncCursor(records, cursor1)
	page2, _, hasMore2, err := paginateSyncRecords(filtered, limit, cursor1)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if hasMore2 {
		t.Fatal("page 2: expected hasMore=false")
	}
	if len(page2) != total-limit {
		t.Fatalf("page 2: got %d records, want %d", len(page2), total-limit)
	}

	seen := make(map[string]bool, total)
	for _, r := range page1 {
		key := r.RecordID.String()
		if seen[key] {
			t.Fatalf("duplicate record %s in page 1", key)
		}
		seen[key] = true
	}
	for _, r := range page2 {
		key := r.RecordID.String()
		if seen[key] {
			t.Fatalf("duplicate record %s across pages", key)
		}
		seen[key] = true
	}
}

func TestPaginateSyncRecords_AcrossPageBoundary(t *testing.T) {
	now := time.Date(2026, 9, 12, 8, 53, 48, 0, time.UTC)

	records := make([]models.SyncPullRecord, 10)
	for i := 0; i < 10; i++ {
		records[i] = models.SyncPullRecord{
			EntityType: "persons",
			RecordID:   uuid.New(),
			Version:    1,
			UpdatedAt:  now,
			IsDeleted:  false,
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].RecordID.String() < records[j].RecordID.String()
	})

	page1, cursor1, hasMore1, err := paginateSyncRecords(records, 5, SyncCursor{})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if !hasMore1 {
		t.Fatal("page 1: expected hasMore=true")
	}
	if len(page1) != 5 {
		t.Fatalf("page 1: got %d records, want 5", len(page1))
	}

	filtered := filterBySyncCursor(records, cursor1)
	page2, _, hasMore2, err := paginateSyncRecords(filtered, 5, cursor1)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if hasMore2 {
		t.Fatal("page 2: expected hasMore=false")
	}
	if len(page2) != 5 {
		t.Fatalf("page 2: got %d records, want 5", len(page2))
	}

	for _, a := range page1 {
		for _, b := range page2 {
			if a.RecordID == b.RecordID {
				t.Fatalf("record %s appears in both pages", a.RecordID)
			}
		}
	}
}

func TestPaginateSyncRecords_DuplicatePrevention(t *testing.T) {
	now := time.Date(2026, 9, 12, 8, 53, 48, 0, time.UTC)

	records := make([]models.SyncPullRecord, 20)
	for i := 0; i < 20; i++ {
		records[i] = models.SyncPullRecord{
			EntityType: "persons",
			RecordID:   uuid.New(),
			Version:    1,
			UpdatedAt:  now,
			IsDeleted:  false,
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].RecordID.String() < records[j].RecordID.String()
	})

	seen := make(map[string]bool)
	cursor := SyncCursor{}
	limit := 7
	pages := 0

	for {
		filtered := filterBySyncCursor(records, cursor)
		page, nextCursor, hasMore, err := paginateSyncRecords(filtered, limit, cursor)
		if err != nil {
			t.Fatalf("page %d: %v", pages+1, err)
		}
		pages++

		for _, r := range page {
			key := r.RecordID.String()
			if seen[key] {
				t.Fatalf("duplicate record %s on page %d", key, pages)
			}
			seen[key] = true
		}

		if !hasMore {
			break
		}
		cursor = nextCursor

		if pages > 10 {
			t.Fatal("too many pages")
		}
	}

	if len(seen) != 20 {
		t.Fatalf("total unique records = %d, want 20", len(seen))
	}
}

func TestPaginateSyncRecords_EmptyPage(t *testing.T) {
	page, cursor, hasMore, err := paginateSyncRecords(nil, 10, SyncCursor{})
	if err != nil {
		t.Fatalf("empty page: %v", err)
	}
	if hasMore {
		t.Fatal("empty page: expected hasMore=false")
	}
	if len(page) != 0 {
		t.Fatalf("empty page: got %d records, want 0", len(page))
	}
	if !cursor.Time.IsZero() || cursor.ID != uuid.Nil {
		t.Fatalf("empty page: cursor should be zero, got %+v", cursor)
	}
}

func TestPaginateSyncRecords_LimitClamping(t *testing.T) {
	records := make([]models.SyncPullRecord, 100)
	for i := 0; i < 100; i++ {
		records[i] = models.SyncPullRecord{
			EntityType: "persons",
			RecordID:   uuid.New(),
			Version:    1,
			UpdatedAt:  time.Date(2026, 9, 12, 8, 53, 48, 0, time.UTC),
			IsDeleted:  false,
		}
	}

	page, _, _, err := paginateSyncRecords(records, 9999, SyncCursor{})
	if err != nil {
		t.Fatalf("clamped limit: %v", err)
	}
	if len(page) != len(records) {
		t.Fatalf("clamped limit: got %d records, want %d", len(page), len(records))
	}
}

func TestSyncCursorWhere_CompositePredicate(t *testing.T) {
	cursor := SyncCursor{
		Time: time.Date(2026, 9, 12, 8, 53, 48, 0, time.UTC),
		ID:   uuid.MustParse("0198f4a2-7c1d-7de2-9f3a-5b6c7d8e9f01"),
	}

	where, args := syncCursorWhere(cursor)
	if where == "" {
		t.Fatal("predicate must not be empty")
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}

	if !strings.Contains(where, "updated_at") || !strings.Contains(where, "id") {
		t.Fatalf("predicate must reference updated_at and id: %s", where)
	}
}

func TestNextSyncCursor_Advances(t *testing.T) {
	now := time.Date(2026, 9, 12, 8, 53, 48, 0, time.UTC)
	id := uuid.MustParse("0198f4a2-7c1d-7de2-9f3a-5b6c7d8e9f01")

	input := SyncCursor{}
	next := nextSyncCursor(input, now, id)

	if !next.Time.Equal(now) {
		t.Fatalf("next cursor time = %v, want %v", next.Time, now)
	}
	if next.ID != id {
		t.Fatalf("next cursor id = %s, want %s", next.ID, id)
	}
}
