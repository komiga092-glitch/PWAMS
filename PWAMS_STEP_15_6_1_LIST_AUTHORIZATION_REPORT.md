# PWAMS — STEP 15.6.1 — List-Endpoint Object-Level Authorization Report

**Date:** 2026-09-11
**Scope:** ListPersons, ListStudents, ListDonations object-level authorization closure
**Predecessor:** PWAMS_STEP_15_6_AUTHORIZATION_REMEDIATION_REPORT.md
**Final verdict:** **PASS** (see §13)

---

## 1. Endpoints audited

Every production list path for Person, Student and Donation was enumerated from
the route registrations down to the SQL query. Each entity has exactly two list
call sites (HTML page + JSON list); there are no others.

| Entity | Route | Handler | Service | Repository | DB query |
|---|---|---|---|---|---|
| Person | `GET /persons/page` | `PersonHandler.Page` (person_handler.go:396) | `ListPersons` | `PersonRepository.List` | GORM scoped SELECT + COUNT |
| Person | `GET /persons` (JSON) | `PersonHandler.List` (person_handler.go:~130) | `ListPersons` | `PersonRepository.List` | GORM scoped SELECT + COUNT |
| Student | `GET /students/page` | `StudentHandler.Page` (student_handler.go:437) | `ListStudents` | `StudentRepository.List` | GORM scoped SELECT + COUNT |
| Student | `GET /students` (JSON) | `StudentHandler.List` (student_handler.go:~115) | `ListStudents` | `StudentRepository.List` | GORM scoped SELECT + COUNT |
| Donation | `GET /donations/page` | `DonationHandler.Page` (donation_handler.go:~50) | `ListDonations` | `DonationRepository.List` | GORM scoped SELECT + COUNT |
| Donation | `GET /donations` (JSON) | `DonationHandler.List` (donation_handler.go:~140) | `ListDonations` | `DonationRepository.List` | GORM scoped SELECT + COUNT |

Authenticated actor at each layer:

- **Handler:** `actorFromContext(c)` (populated by `RequireAuth`) — all six call
  sites now resolve the actor and pass it to the service. Previously the actor
  was resolved but ignored by the list handlers.
- **Service:** `Actor{ID, Role}` parameter; authorization decision made here via
  `ownershipFilter(actor)`.
- **Repository:** receives a concrete `ownerID uuid.UUID` scope; applies
  `created_by_id = ?` at the database level (or no predicate for privileged).
- **Database:** unauthorized rows are never returned by the query — no
  post-fetch filtering in Go.

No CSV/export endpoint exists anywhere in the Go codebase (codebase-wide
search for export/CSV found only TypeScript module exports and historical
report text). Exports are **N/A**; see §8.

## 2. Previous vulnerability (from STEP 15.6)

STEP 15.6 protected single-record reads/writes with `CanAccessRecord`, but the
three list endpoints loaded **all rows** regardless of caller:

1. List handlers resolved the actor but called the service without any
   ownership scope.
2. Repositories had no `created_by_id` predicate in list queries.
3. Count queries used for pagination metadata counted all rows, leaking
   aggregate information about other users' records.

A non-privileged user could therefore enumerate other users' records via
pagination, sorting, search, or filter parameters.

**Additional defect discovered and fixed during this step:** the pre-existing
search predicates used unparenthesized `OR` groups, e.g.
`LOWER(full_name) LIKE ? OR LOWER(nic_passport) LIKE ? ...`. Combined with the
new `AND created_by_id = ?` scoping, SQL precedence (`AND` binds tighter than
`OR`) produced `(ownership AND first_field) OR other_field ...`, letting a
search match leak rows the actor does not own. The new integration tests
failed on exactly this case (`search bypass`) and the fix (parenthesizing the
OR groups) was applied to all three repositories. The same latent pattern
still exists in `donor_repository.go` and `aid_request_repository.go` (see
§12).

## 3. Authorization model

Reuses the existing STEP 15.6 model — no new roles, tables, or migrations:

- `IsPrivilegedRole(role)` — Super Admin, Admin, Partner/Manager, Staff,
  Volunteer retain unrestricted list access (existing legitimate behavior).
- `CanAccessRecord(actor, ownerID)` — unchanged; governs single-record access.
- **New helper `ownershipFilter(actor)` (services/ownership.go:76):**
  - `actor.ID == uuid.Nil` → `uuid.Nil` (fail-closed: no rows — an
    unauthenticated actor must see nothing).
  - Privileged role → `uuid.Nil` meaning "no restriction" (full access
    preserved).
  - Non-privileged role → `actor.ID`, so the repository query is filtered to
    records the actor created.

Resulting matrix (service-layer enforcement, independent of route RBAC):

| Role | ListPersons | ListStudents | ListDonations |
|---|---|---|---|
| Super Admin | all rows | all rows | all rows |
| Admin | all rows | all rows | all rows |
| Manager/Partner | all rows | all rows | all rows |
| Staff | all rows | all rows | all rows |
| Volunteer | all rows | all rows | all rows |
| Donor | own only | own only | own only |
| Beneficiary | own only | own only | own only |
| Student | own only | own only | own only |

Route-level RBAC (unchanged, first gate) additionally restricts who reaches
these endpoints at all: `/persons` and `/donations` admit only privileged
roles; `/students` additionally admits RoleStudent. The list scoping is the
second, defense-in-depth gate and is enforced even if route RBAC changes.

## 4. Files changed (production)

| File | Change |
|---|---|
| `internal/services/ownership.go` | Added `ownershipFilter(actor)` — converts actor + privilege into a DB-level owner scope (uuid.Nil = unrestricted). |
| `internal/services/person_service.go` | `ListPersons` computes the scope and passes `ownerID` to the repository. |
| `internal/services/student_service.go` | `ListStudents` — same. |
| `internal/services/donation_service.go` | `ListDonations` — same. |
| `internal/repository/person_repository.go` | `List` accepts `ownerID uuid.UUID`; applies `created_by_id = ?` when non-nil to both the row query and the COUNT query; parenthesized the search `OR` group. |
| `internal/repository/student_repository.go` | Same. |
| `internal/repository/donation_repository.go` | Same. |
| `internal/handlers/person_handler.go` | `Page` and `List` now resolve the actor and pass it to `ListPersons`. |
| `internal/handlers/student_handler.go` | `Page` and `List` — same. |
| `internal/handlers/donation_handler.go` | `Page` and `List` — same. |

No new tables, no migrations, no role changes, no architecture changes. All
scoping is applied **inside the SQL query** — unauthorized rows are never
loaded into application memory.

## 5. Tests added (exact)

All new files live in `internal/services` (package `services_test`), use the
existing live-PostgreSQL helpers (`AcquireTestDB`, `SkipUnlessForceIntegration`)
and isolated fixtures (`newFixture` with deferred cleanup). **No existing tests
were modified.**

| File | Test | Covers |
|---|---|---|
| `authz_list_person_test.go` | `TestPersonListObjectLevelAuthorization` | owner sees own (2 rows); non-owner sees only own; empty result for unauthorized actor; Admin delta baseline; pagination scoped; count scoped; search cannot bypass (NIC, for empty + intruder actors); owner search works; status filter cannot bypass; sort/pagination sweep; object-level ID cross-checks. |
| `authz_list_student_test.go` | `TestStudentListObjectLevelAuthorization` | owner sees own; non-owner (Beneficiary) cannot enumerate; empty result; Admin retains; pagination/count scoped; search (student_code) cannot bypass; grade filter cannot bypass; sort sweep; object-level ID cross-checks. |
| `authz_list_donation_test.go` | `TestDonationListObjectLevelAuthorization` | owner (Donor) sees own; **another Donor** cannot enumerate; **Beneficiary** cannot enumerate; **Student** cannot enumerate; Admin retains; pagination/count scoped; search (reference_no) cannot bypass; type & status filters cannot bypass; sort sweep; object-level ID cross-checks. |
| `authz_list_roles_test.go` | `TestListAuthorization_PrivilegedRolesRetainAccess` | Super Admin, Admin, Staff, Volunteer, Partner/Manager each find a record they did not create (search-scoped, exact count) — privileged full access preserved. |
| `authz_list_nonpriv_test.go` | `TestListAuthorization_NonPrivilegedRolesAreScoped` | Donor, Beneficiary, Student each get 0 rows/total 0 against staff-owned records. |
| `authz_list_nonpriv_test.go` | `TestListAuthorization_OwnerSeesOwnRecordsOnly` | Two distinct non-privileged owners each see exactly their own record and never each other's. |

## 6. PostgreSQL test evidence (live execution)

Database: live PostgreSQL reachable at `localhost:5432` (docker), database
`pwams_db`, application user `pwams_user` (verified with psql:
`SELECT current_user` → `pwams_user`). Tests ran with
`PWAMS_FORCE_INTEGRATION=1`; **0 tests skipped**.

Focused selection (`go test -count=2 -v -run 'ListAuthorization|PersonList|StudentList|DonationList' ./internal/services`):

```
--- PASS: TestDonationListObjectLevelAuthorization (1.53s)
--- PASS: TestListAuthorization_NonPrivilegedRolesAreScoped (0.04s)
--- PASS: TestListAuthorization_OwnerSeesOwnRecordsOnly (0.03s)
--- PASS: TestPersonListObjectLevelAuthorization (0.08s)
--- PASS: TestListAuthorization_PrivilegedRolesRetainAccess (0.11s)
--- PASS: TestStudentListObjectLevelAuthorization (0.08s)
--- (each test executed twice via -count=2 — all PASS)
ok  github.com/komiga092-glitch/pwams/internal/services 2.587s
```

Full required run (`PWAMS_FORCE_INTEGRATION=1 go test -count=2 -v ./internal/services ./internal/handlers`):

```
ok  github.com/komiga092-glitch/pwams/internal/services   4.410s
ok  github.com/komiga092-glitch/pwams/internal/handlers   0.393s
PASS: 84, FAIL: 0, SKIP: 0
```

## 7. Pagination / count verification

Verified directly by the tests above, at the service/repository boundary that
feeds both the JSON responses and the HTML pages:

- Owner with 2 records, `PageSize=1`, `Page=1`: exactly 1 row, `total=2`,
  `page=1`, `pageSize=1` — the total reflects **only authorized rows**.
- Pagination sweep: walking pages 1..2 at `PageSize=1` yields owned records
  only — no unauthorized row appears on any page.
- Unauthorized actor (owns nothing): `total=0`, 0 rows — if Actor A owns 2 and
  Actor B owns 100, Actor A receives `total=2` for their own scope, never
  `total=102`. Count is computed by a scoped `COUNT` inside the same WHERE
  predicates as the row query.
- Because both the row query and the count query apply the identical
  `created_by_id` scope (plus identical search/filter predicates), page
  metadata can never disclose the existence of unauthorized records.

## 8. Search / filter / export verification

- **Search cannot bypass ownership:** the owner's unique search key
  (Person → `nic_passport`, Student → `student_code`, Donation →
  `reference_no`) returns **0 rows / total 0** for actors with no authorized
  records (empty actor and cross-role intruder actors) and still returns
  exactly 1 row for the owner. Enforced at SQL level: the parenthesized
  search group is ANDed with the `created_by_id` scope.
- **Filters cannot bypass ownership:** status filter (Person), grade filter
  (Student), type and status filters (Donation) combined with non-privileged
  actors yield 0 rows / total 0. Filters are ANDed inside the same scoped
  WHERE clause.
- **Sort cannot bypass ownership:** ordering is fixed server-side
  (`created_at DESC`); client-supplied sort columns are not accepted. The
  tests additionally sweep every page of the sorted result and assert every
  returned row is owned.
- **Export:** no CSV/JSON export endpoint exists in the Go codebase (searched
  `export`/`csv` across all Go, handler and route files; only TypeScript
  module exports and historical report prose matched). N/A — any future
  export must call the same scoped service methods.
- **HTML list pages:** `persons.html`, `students.html`, `donations.html` (and
  their templ components) render exactly the slice returned by the scoped
  service calls through the `Page` handlers — same dataset, same scoping, no
  separate data path.

## 9. Negative authorization cases (all verified on live PostgreSQL)

1. Donor owner's Person records: not visible to a Beneficiary intruder, not
   findable by NIC search, not counted, not reachable on any page.
2. Donor owner's Student records: not visible to a Beneficiary intruder, not
   findable by student_code search, not counted.
3. Donor owner's Donations: not visible to **another Donor**, not visible to a
   **Beneficiary**, not visible to a **Student** — by list, by search, by
   filter, by count, and by direct ID lookup (`ErrRecordAccessDenied`).
4. Actor with zero authorized records receives `total=0` and empty rows on
   every query shape (plain, search, filter, deep pages).
5. Object-level regression checks re-confirmed: non-owners get
   `ErrRecordAccessDenied` on `GetPersonByID` / `GetStudentByID` /
   `GetDonationByID`.

## 10. Privileged / non-privileged role verification

- **Privileged (existing model, unchanged):** `TestListAuthorization_
  PrivilegedRolesRetainAccess` proves Super Admin, Admin, Staff, Volunteer and
  Partner/Manager each retrieve a record they did not create (exact search
  count = 1). Admin full-access behavior additionally asserted via
  baseline-delta in each entity test.
- **Non-privileged:** `TestListAuthorization_NonPrivilegedRolesAreScoped`
  proves Donor, Beneficiary and Student each receive 0 rows against
  staff-owned records; `TestListAuthorization_OwnerSeesOwnRecordsOnly` proves
  mutual isolation between two non-privileged owners.
- Route RBAC unchanged (preserves Manager/Partner compatibility): Partner
  remains on all three route groups; Student remains on `/students` only, and
  the service now scopes that Student to its own records.

## 11. Build / vet / test results

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 (clean) |
| `gofmt -l` (repository, services, handlers, models) | clean |
| `go test -count=2 ./...` (integration skipped) | exit 0 — handlers, middleware, models, services, utils all `ok` |
| `PWAMS_FORCE_INTEGRATION=1 go test -count=2 ./internal/services ./internal/handlers` | exit 0 — **84 PASS / 0 FAIL / 0 SKIP** on live PostgreSQL |

The initial `go test` run failed with three "search bypass" failures — this
was the SQL-precedence defect described in §2, caught by the new tests and
fixed; subsequent runs are green, including `-count=2`.

## 12. Alternate-route / bypass audit (task item 12)

- `ListPersons`, `ListStudents`, `ListDonations` are called from exactly the
  six handler call sites in §1 — no other production callers exist.
- Route files audited (`person_routes.go`, `student_routes.go`,
  `donation_routes.go`, plus `donor_routes.go`, `aid_request_routes.go` and
  the reports group): no second list route per entity, no HTMX or JSON
  duplicate path that skips the service, no CSV/export route.
- Repository surface audited: `PersonRepository`, `StudentRepository`,
  `DonationRepository` expose exactly one list method each (`List`, now
  scoped); remaining methods are single-record operations already guarded by
  `CanAccessRecord` in STEP 15.6 (`FindByID`, `Create`, `Update`,
  `UpdateStatus`, `SoftDelete`, `Exists*`).
- Reports endpoints remain Super Admin/Admin-only (unchanged, separate
  surface).

## 13. Remaining security gaps (follow-up candidates — not started, per scope)

1. **Aid-request list scoping (recommended next):** `/aid-requests` admits
   non-privileged Beneficiary and Student roles, and
   `aid_request_repository.go` still uses the same unparenthesized search
   `OR` pattern; whether its list is ownership-scoped was **not verified in
   this step** (out of scope). Highest-priority candidate for a future step.
2. **Donor list hardening (low):** `donor_repository.go` shares the
   unparenthesized search pattern, but `/donors` is privileged-only
   (Super Admin/Admin/Staff/Partner), so it is not exploitable for cross-user
   enumeration today; still worth parenthesizing for defense-in-depth.
3. **Future exports:** if CSV/export endpoints are added, they must consume
   the scoped service methods — enforced by convention, not by code today.
4. Report endpoints unchanged and privileged-only; no CSV export exists.

## 14. Final verdict

**PASS**

- All six list call sites enforce object-level authorization at the database
  level (no post-fetch filtering).
- Pagination, count, search, filter and sort are all scoped; pagination
  metadata cannot leak other users' record counts.
- Privileged roles (Super Admin, Admin, Manager/Partner, Staff, Volunteer)
  retain full access; non-privileged roles (Donor, Beneficiary, Student) are
  isolated to their own records.
- Live PostgreSQL integration evidence: 84 PASS / 0 FAIL / 0 SKIP
  (`-count=2`), including all negative cases.
- A pre-existing SQL-precedence search defect was discovered by the new tests
  and fixed in the three audited repositories; residual instances outside this
  step's scope are listed in §13.
- STEP 15.7 intentionally **not** started.

<!-- STEP 15.6.1 END -->
