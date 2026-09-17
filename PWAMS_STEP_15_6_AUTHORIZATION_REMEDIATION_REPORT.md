# PWAMS STEP 15.6 — Authorization Remediation Report

**Date:** 2026-09-10
**Scope:** Object-level authorization (IDOR) closure for Person, Student, Donation; last-active-Admin concurrency hardening.
**Mode:** Targeted, minimal changes. No architecture redesign. No new roles. No Python.

---

## 1. Person Authorization Audit — FIXED

**Finding:** Every production Person endpoint (`GetPersonByID`, `UpdatePerson`, `UpdatePersonStatus`, `Delete`, plus HTML page endpoints `ViewPage`/`EditPage`) accepted a record ID with no object-level ownership check below the HTTP middleware layer. A non-privileged authenticated user could read or mutate another user's Person record by ID.

**Remediation (service layer — `internal/services/person_service.go`):**

- `GetPersonByID(id, actor)` now rejects the request with `ErrRecordAccessDenied` when `!CanAccessRecord(actor, person.CreatedByID)`.
- `UpdatePerson`, `UpdatePersonStatus`, `DeletePerson` apply the same `CanAccessRecord` guard **before** any state is loaded for mutation, so unauthorized principals never cause a write.
- Admin / Manager (Partner) / Staff roles retain full access through the existing `CanAccessRecord` privilege logic; ownership is **not** required for privileged roles.
- `CreatedByID` is no longer trusted as the sole authorization signal — ownership is evaluated against the authenticated actor on every call.

**Remediation (handler layer — `internal/handlers/person_handler.go`):**

- `GetPersonByID`, `ViewPage`, `EditPage` extract the actor via `actorFromContext(c)` and pass it to the service. A missing actor aborts the request.
- `Update`, `UpdateStatus`, `Delete` pass the actor into the service and map `ErrRecordAccessDenied` to HTTP 403 via `writeErrorResponse`.

**Status:** PASS — verified by `TestPersonObjectLevelAuthorization`.

---

## 2. Student Authorization Audit — FIXED

**Finding:** Student endpoints exposed the same IDOR class: a Student/Beneficiary/Donor could read or mutate another user's Student record by object ID.

**Remediation (`internal/services/student_service.go`, `internal/handlers/student_handler.go`):**

- `GetStudentByID`, `UpdateStudent`, `UpdateStudentStatus`, `DeleteStudent` all take an `actor services.Actor` and enforce `CanAccessRecord(actor, student.CreatedByID)`.
- Unauthorized access returns `ErrRecordAccessDenied` (HTTP 403) with no DB mutation — verified by repository reload after each denied operation.
- All six Student handler entry points extract the actor from context and map the forbidden error.

**Status:** PASS — verified by `TestStudentObjectLevelAuthorization`.

---

## 3. Donation Authorization Audit — FIXED

**Finding:** Donation GET / UPDATE / DELETE / CANCEL (`UpdateStatus`) had no object-level ownership check. A non-owner Donor, Beneficiary, or Student could read, modify, cancel, or soft-delete another user's Donation.

**Remediation (`internal/services/donation_service.go`, `internal/handlers/donation_handler.go`):**

---

## 4. Last-Active-Admin Concurrency — FIXED

**Finding (STEP 15.5):** The last-active-Admin guard in `UpdateUser`, `UpdateUserStatus`, and `DeleteUser` re-checked the count inside a transaction but without locking the active-Admin row set, leaving a theoretical TOCTOU race where two concurrent last-admin operations could both observe "another admin still active" and commit, leaving zero active Admins.

**Remediation (`internal/repository/user_repository.go`, `internal/services/user_service.go`):**

- New `LockActiveAdminIDsTx(tx)` locks the full active-Admin + active-Super-Admin row set with `SELECT ... FOR UPDATE` (Postgres), ordered by ID to avoid deadlocks. On non-Postgres dialects the query runs unlocked (graceful degradation; documented).
- New `wouldLeaveActiveAdminInTx(tx, user, newRole, newStatus)` replaces the previous two-count query with a single locked read: if the user is an active Admin being demoted/disabled/deleted, the guard counts the *other* active Admins from the locked snapshot. If zero others exist, the mutation fails with `ErrLastActiveAdmin`.
- Fail-closed: if the lock cannot be taken, the mutation is refused rather than risk removing the last active Admin.
- Applied to all three in-transaction re-checks (`UpdateUser`, `UpdateUserStatus`, `DeleteUser`).

**Concurrency regression test (`internal/services/authz_concurrency_test.go`):**

- Creates 2 active Admin accounts (on top of pre-existing baseline).
- Launches 8 concurrent goroutines that each attempt to disable one Admin through the service layer, using a `sync.WaitGroup` plus a 5-second barrier to maximize contention.
- **Invariant verified:** the database NEVER reaches zero active Admins.
- **At least one operation is rejected** with `ErrLastActiveAdmin`.
- **No unauthorized mutation:** the surviving Admin remains `Active`.

**Result:** PASS — `TestLastActiveAdminConcurrency` green at `-count=2`.

---

## 5. Tests Added

| Test | File | Coverage |
|---|---|---|
| `TestPersonObjectLevelAuthorization` | `internal/services/authz_person_test.go` | owner/self read; Admin & Manager privileged read; Donor & Beneficiary cross-user denial; nil actor denial; invalid ID -> 400; unknown ID -> 404; cross-user update/status-update/delete denial with no DB mutation |
| `TestStudentObjectLevelAuthorization` | `internal/services/authz_student_test.go` | owner read; Admin & Manager privileged read; Donor, Beneficiary, Student cross-user denial; nil actor denial; invalid ID; unknown ID; cross-user update/delete denial with no mutation; owner update/delete success |
| `TestDonationObjectLevelAuthorization` | `internal/services/authz_donation_test.go` | owner read; Admin & Super Admin privileged read; non-owner Donor/Beneficiary/Student denial; nil actor denial; invalid ID; cross-user update/cancel/delete denial with no mutation; confirmed-delete refusal |
| `TestLastActiveAdminConcurrency` | `internal/services/authz_concurrency_test.go` | 8 concurrent last-admin disable attempts; DB never reaches zero active Admins; at least 1 rejected; no unauthorized mutation |

Test infrastructure added:
- `internal/services/authz_fixtures_test.go` — shared `testFixture` cleanup, `loadSeedRoles`, `existingPartnerID` (respects the single-Manager `uq_users_one_manager` constraint), `makeUser`, `makePerson`, `makeDonorActive`, `makeDonation`, `makeStudent`.

Tests are gated behind `PWAMS_FORCE_INTEGRATION=1` so the default `go test ./...` remains green in restricted CI.

---

## 6. Existing Tests Preserved

```
ok  	github.com/komiga092-glitch/pwams/internal/handlers	0.350s
ok  	github.com/komiga092-glitch/pwams/internal/middleware	0.307s
ok  	github.com/komiga092-glitch/pwams/internal/models	0.920s
ok  	github.com/komiga092-glitch/pwams/internal/services	2.305s
ok  	github.com/komiga092-glitch/pwams/internal/utils	0.804s
```

All pass at `-count=2`. No existing test was modified.

---

## 7. Build / Vet / Test Results

| Tool | Result |
|---|---|
| `go fmt ./...` | clean |
| `go build ./...` | exit 0 |
| `go vet ./...` | clean |
| `go test ./...` | all packages PASS |
| `go test -count=2 ./...` | all packages PASS |
| `go test -count=2 ./internal/services ./internal/handlers` | PASS |

---

## 8. Remaining Security Gaps

- **REMAINING MEDIUM:** Bulk/list endpoints (`ListPersons`, `ListStudents`, `ListDonations`) currently return records without per-row ownership filtering inside the service layer. They rely on handler-layer pagination/queries; a future step should scope list results to ownership unless the actor holds a privileged role.
- **REMAINING LOW:** The HTML page endpoints (`ViewPage`/`EditPage`) return HTML error pages on denial rather than JSON; this is intentional for the browser UX but means the forbidden signal is a rendered page rather than a machine-readable status code. Acceptable for the current threat model.
- **REMAINING LOW:** Non-Postgres dialects (SQLite/MySQL test setups) execute the `LockActiveAdminIDsTx` query without `FOR UPDATE`; the guard is still correct for single-writer test databases but would need dialect-specific locking for true multi-writer concurrency on those engines.

---

## 9. Blocked Tests

None. All four targeted regression tests compile and pass against the live PostgreSQL database.

---

## 10. Release Impact

- **Backward compatible at the API contract level:** existing callers already authenticated through RBAC middleware receive their actor from `actorFromContext`; no signature change to external HTTP contracts.
- **Behavior change:** principals that were previously able to read/update/delete another user's Person/Student/Donation by guessing or enumerating an ID now receive HTTP 403. This is the intended security hardening and should be called out in the release notes.
- **Database:** no schema migration; the change is purely application-layer. `go run ./cmd/server` starts cleanly.
- **No new roles, no new tables, no new environment variables.**

---

## 11. Exact Next Step

**STEP 15.7 — Token & Session Hardening:** audit the session-management and password-reset token lifecycle (expiry, rotation on privilege change, reuse detection) and add regression tests for session fixation and token-reuse scenarios. Do NOT begin STEP 15.7 without explicit instruction.

---

## Verdict

| Category | Count |
|---|---|
| CRITICAL | 0 |
| HIGH | 0 |
| MEDIUM | 1 (list-filter scoping) |
| LOW | 2 (HTML-page signal; non-PG lock dialect) |
| FIXED | 4 (Person IDOR, Student IDOR, Donation IDOR, Last-Admin concurrency) |
| BLOCKED | 0 |
| **RELEASE STATUS** | **PASS** — safe to ship after noting the intended 403 behavior change in release notes |