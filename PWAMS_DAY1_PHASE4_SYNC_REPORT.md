# PWAMS — DAY 1 / PHASE 4 — SYNC CURSOR CORRECTNESS REPORT

Date: 2026-09-13
Scope: Fix sync pagination correctness using deterministic composite cursor

## 1. Problem

The previous sync pull used `updated_at` as the sole pagination cursor. When multiple records shared the same `updated_at` timestamp, the keyset predicate `updated_at > cursor_time` would skip every unsent record inside that timestamp group once the page limit was reached. Records with identical timestamps could be silently lost during multi-page sync.

## 2. Solution

Replace the time-only cursor with a **deterministic composite cursor** `(updated_at, id)`:

- **Ordering**: `ORDER BY updated_at ASC, id ASC`
- **Next-page predicate**: `updated_at > ? OR (updated_at = ? AND id > ?)`
- **Cursor format**: `<RFC3339Nano>~<uuid>` (e.g., `2026-09-12T08:53:48.262648Z~0198f4a2-7c1d-7de2-9f3a-5b6c7d8e9f01`)
- **Legacy compatibility**: A bare RFC3339 timestamp is accepted for backward compatibility. With the last-seen record id unknown, the safe reading is "re-serve every record sharing that timestamp" (the client upserts idempotently), never "skip them."
- **Invalid cursor**: Returns HTTP 400.
- **Limit clamping**: Hard upper bound of 500 records per page (`SyncPullMaxLimit`).

## 3. Files Modified

| File | Change |
|---|---|
| `internal/repository/sync_repository.go` | Added `SyncCursor` struct, `EncodeSyncCursor`, `ParseSyncCursor`, `clampSyncLimit`, `syncCursorWhere`, `nextSyncCursor`, `paginateSyncRecords` functions. Updated `PullEntities` to use composite cursor. |
| `internal/services/sync_service.go` | Updated to pass composite cursor through the service layer. |
| `internal/handlers/sync_handler.go` | Updated to parse and validate the composite cursor from the query string. |

## 4. Files Added

| File | Purpose |
|---|---|
| `internal/repository/sync_pagination_test.go` | 12 unit tests covering cursor encoding/parsing, limit clamping, and pagination correctness. |

## 5. Test Results

All 12 sync pagination tests pass:

| Test | Result |
|---|---|
| `TestParseSyncCursor_EmptyIsFirstPage` | PASS |
| `TestParseSyncCursor_CompositeRoundTrip` | PASS |
| `TestParseSyncCursor_LegacyTimeOnly` | PASS |
| `TestParseSyncCursor_Invalid` | PASS |
| `TestClampSyncLimit` | PASS |
| `TestPaginateSyncRecords_600IdenticalTimestamps` | PASS |
| `TestPaginateSyncRecords_AcrossPageBoundary` | PASS |
| `TestPaginateSyncRecords_DuplicatePrevention` | PASS |
| `TestPaginateSyncRecords_EmptyPage` | PASS |
| `TestPaginateSyncRecords_LimitClamping` | PASS |
| `TestSyncCursorWhere_CompositePredicate` | PASS |
| `TestNextSyncCursor_Advances` | PASS |

## 6. Verification Results

| Command | Result |
|---|---|
| `go fmt ./...` | PASS (exit 0) |
| `go build ./...` | PASS (exit 0) |
| `go vet ./...` | PASS (exit 0) |
| `go test ./...` | PASS (all packages) |
| `npm run build` | PASS (exit 0) |

## 7. Guarantees

- **First page works**: Empty cursor returns records from the beginning.
- **Next page works**: Composite cursor correctly identifies the next boundary.
- **Exactly 500 limit maximum**: `clampSyncLimit` enforces `SyncPullMaxLimit`.
- **No duplicate records**: The `id > cursor_id` clause within the same timestamp prevents re-serving.
- **No skipped records**: The `OR (updated_at = ? AND id > ?)` clause captures all records at the boundary timestamp.
- **Invalid cursor returns 400**: `ParseSyncCursor` rejects malformed cursors.
- **Old cursor compatibility**: Legacy time-only cursors are accepted and handled safely.
- **No arbitrary entity/table injection**: Table names are hardcoded in the repository, not user-controlled.
- **Existing ownership and RBAC rules remain unchanged**: The pagination change does not affect access control.

## 8. Remaining Blockers

None. Phase 4 is complete.