# PWAMS — STEP 15.6.3 REPORT
## Loan Authorization, IDOR & Search/Filter Bypass Hardening

Status legend: PASS / FAIL / PARTIAL / BLOCKED / NOT VERIFIED
Scope rule honored: **STEP 15.7 was not started.** No TLS, npm, session/token,
WebSocket, JWT-localStorage, or Python changes were made.

---

## 1. Executive Summary

The Loan and Loan Repayment authorization surface was fully enumerated,
the ownership model documented (existing `loans.created_by_id`; no new
column), and three confirmed gaps closed:

1. **Unscoped Loan list** — `GET /loans` returned every row in the table
   to every role in the route allow-list (including Beneficiary and
   Student). The repository `List` now receives a DB-level owner scope
   and applies `created_by_id = ?` before BOTH the COUNT and the row
   query. Loan repayments are scoped through the parent loan's owner
   using a SQL `EXISTS` predicate.
2. **Unparenthesized OR search condition** —
   `LOWER(...) LIKE ? OR LOWER(...) LIKE ? OR LOWER(purpose) LIKE ?` was
   vulnerable to SQL operator-precedence reasoning and is now wrapped in
   explicit parentheses (defense in depth; GORM v1.31.2 happens to wrap
   multi-expression `Expr`s, but the code no longer relies on ORM
   internals).
3. **Fail-open nil-actor `ownershipFilter`** — the previous STEP 15.6.1
   report claimed nil-actor behavior was fail-closed; the code actually
   returned `uuid.Nil` (= *unrestricted*) for a nil actor. This was a
   **corrected finding**. `ownershipFilter` now returns
   `(uuid.UUID, bool)` and every caller fails closed with
   `ErrRecordAccessDenied` before touching the repository.

Live PostgreSQL integration tests (real DB, `PWAMS_FORCE_INTEGRATION=1`)
cover owner/non-owner/privileged matrices, list/count isolation,
search/filter/pagination bypass attempts, repayment IDOR, mutation-safety
before/after DB assertions, nil-actor, invalid-UUID and unknown-ID paths.
All PASS.

## 2. Exact Loan Route Inventory (Phase 1)

Call graph (route → handler → service → repository):

| Route | Handler | Service | Repository |
|---|---|---|---|
| `GET /loans/page` | `LoanHandler.Page` | `LoanService.ListLoans` | `LoanRepository.List` |
| `POST /loans` | `LoanHandler.Create` | `LoanService.CreateLoan` | `LoanRepository.Create` |
| `GET /loans` (JSON list) | `LoanHandler.List` | `LoanService.ListLoans` | `LoanRepository.List` |
| `GET /loans/:id` | `LoanHandler.GetByID` | `LoanService.GetLoanByID` | `LoanRepository.FindByID` |
| `PATCH /loans/:id/review` (Super Admin/Admin only) | `LoanHandler.Review` | `LoanService.ReviewLoan` | `FindByID` + `Update` |
| `GET /loan-repayments/page` | `LoanRepaymentHandler.Page` | `LoanRepaymentService.List` | `LoanRepaymentRepository.List` |
| `POST /loan-repayments` | `LoanRepaymentHandler.Create` | `LoanRepaymentService.Create` | `LoanRepository.FindByID`, `RepaymentRepository.Create` |
| `GET /loan-repayments` | `LoanRepaymentHandler.List` | `LoanRepaymentService.List` | `LoanRepaymentRepository.List` |
| `GET /loan-repayments/:id` | `LoanRepaymentHandler.GetByID` | `LoanRepaymentService.GetByID` | `FindByID` + `authorizeRepaymentAccess` |
| `PATCH /loan-repayments/:id/pay` | `LoanRepaymentHandler.Pay` | `LoanRepaymentService.Pay` (tx + CAS) | `UpdatePaymentGuarded`, `HasOutstandingRepayments`, `LoanRepository.Update` |
| `PATCH /loan-repayments/:id/cancel` | `LoanRepaymentHandler.Cancel` | `LoanRepaymentService.Cancel` | `FindByID` + `Update` |

Route RBAC groups (middleware): Super Admin, Admin, Staff, Partner,
Beneficiary, Student for both groups; Review additionally restricted to
Super Admin/Admin at the route level (service re-checks ownership).

**Endpoints that do NOT exist** (verified by code search — no callers):
loan delete, loan update/edit, loan cancel endpoint, repayment
update/delete endpoints, repayment status endpoint. The repository
`Delete` methods are unused by HTTP (used only by test fixture cleanup).
No export/download endpoint exposes loans. Dashboard exposes only an
aggregate `ActiveLoans` count (`CountWithoutDeleted`), not records.

**Alternate routes:**
- `POST /api/v1/sync/push` and `GET /api/v1/sync/pull`
  (`loan` / `loan_repayment` entities) — restricted to Super Admin,
  Admin, Staff; server-authoritative `CreatedByID`; tenant isolation;
  version CAS on update/delete. Privileged-only surface
  (NOT VERIFIED over HTTP in this step; code-reviewed only).
- `GET /reports/loans` exists only as a UI link in a template — there is
  **no registered route** in `internal/routes/report_routes.go` (only
  dashboard/donations/aid-requests). Clicking it yields 404. No leak.
- `MarkOverdue` repayment sweep: system job, not HTTP-exposed; passes
  unrestricted scope by design (commented).

## 3. Exact Loan Ownership Model (Phase 2)

- **Loan ownership** = `loans.created_by_id` (existing column,
  `NOT NULL`, set from the authenticated principal in `CreateLoan`;
  server-authoritative in sync).
- **Loan Repayment ownership** = inherited — resolved through the parent
  loan's `created_by_id`. There is no owner column on
  `loan_repayments` and **no new column was invented** (per instructions).
- **Inheritance/derivation**: single-record repayment ops call
  `authorizeRepaymentAccess(loanID, actor)` → `LoanRepository.FindByID`
  → `CanAccessRecord(actor, loan.CreatedByID)`. List ops apply a
  DB-level `EXISTS (SELECT 1 FROM loans WHERE loans.id =
  loan_repayments.loan_id AND loans.created_by_id = ?)` predicate.

## 4. Authorization Matrix (Phase 3 — live-tested unless noted)

| Case | Result |
|---|---|
| A. Super Admin | PASS — full access per privileged policy (service-level, tested) |
| B. Admin | PASS — full access per privileged policy (tested) |
| C. Owner | PASS — owner reads own loan / own repayments (tested) |
| D. Non-owner same-role (Beneficiary→Beneficiary) | PASS — DENIED `ErrRecordAccessDenied` (tested) |
| E. Beneficiary → another user's Loan | PASS — DENIED (tested) |
| F. Student → another user's Loan | PASS — DENIED (tested) |
| G. Staff/Manager (Partner) → another owner's Loan | PASS — allowed per existing privileged RBAC, tested |
| H. nil actor / missing actor | PASS — DENIED, fails closed in service before any repo call (tested with nil repos) |
| I. invalid UUID | PASS — `ErrInvalidLoanID` / `ErrInvalidLoanRepaymentID` (400-class), no panic (tested) |
| J. unknown record ID | PASS — `ErrLoanNotFound` / `ErrLoanRepaymentNotFound` (404-class) (tested) |
| Create (loan) | PASS — route RBAC + service validation; ownership stamped server-side |
| Review/status change | PASS — service-level `CanAccessRecord` denies non-owner non-privileged (tested); route additionally Admin+ |
| Cancel/delete loan | NOT VERIFIED as endpoint — endpoints do not exist (repository method unused) |

Authorization exists at the **service layer** for every path; route
middleware is not relied upon.

## 5. Loan List DB-Level Scoping (Phase 4) — PASS

- Service computes `ownerID, ok := ownershipFilter(actor)`; `!ok` →
  `ErrRecordAccessDenied` (nil actor fails closed, **before** any query).
- Repository applies `created_by_id = ?` when `ownerID != uuid.Nil`,
  before `Count` AND before `Find` — identical predicate on both.
- Privileged actors receive `uuid.Nil` = no predicate (existing policy).
- Live tests prove: row query and COUNT leak nothing (owner sees
  exactly 2/2; intruder sees 1/1 own; student sees 0/0; out-of-range
  page still returns correct total 2).

## 6. Search/Filter/Sort SQL Precedence (Phase 5) — PASS

- Search OR-group now explicitly parenthesized:
  `(LOWER(CAST(id AS TEXT)) LIKE ? OR LOWER(CAST(person_id AS TEXT)) LIKE ? OR LOWER(purpose) LIKE ?)`.
- Ownership predicate is added via a **separate** `.Where()` call, so it
  is AND-composed regardless of internal expression shape.
- Live tests: search with owner's unique purpose token returns 0 rows /
  count 0 for Beneficiary/Student/Donor intruders, and 1 row for the
  owner. `person_id` and `status` filters cannot bypass ownership.
- Sorting is fixed server-side (`created_at DESC` / `due_date ASC`);
  pagination sweep returns only owned rows.
- Repayment list: `loan_id` filter with another user's loan ID returns
  0 rows / count 0 (IDOR via filter denied); invalid loan_id and invalid
  status are rejected with 400-class errors.

## 7. Loan Repayment Authorization (Phase 6) — PASS

- Get / List / Create / Pay / Cancel all enforce ownership at the
  service layer through the parent loan's `CreatedByID`.
- Live-tested: repayment Get, Pay, Cancel and Create-by-loan-ID are all
  denied for a non-owner Beneficiary; no repayment list row of another
  user's loan is ever returned; count never leaks.

## 8. Mutation Safety (Phase 7) — PASS (live DB before/after assertions)

For every denied operation (pay, cancel, repayment-create, loan-review):
- before/after snapshot of `status`, `paid_amount`, `version` is
  identical → no DB mutation, no version increment, no status change,
  no repayment created (count stays 1 after denied create).
- No audit entry is written for denied mutations (audit only fires on
  the success path in `Pay`).

## 9. Privileged Role Matrix (Phase 8) — PASS (code-verified + tested)

| Role | Loan list | Single record | Notes |
|---|---|---|---|
| Super Admin | all rows | any | privileged |
| Admin | all rows | any | privileged |
| Partner (Manager) | all rows | any | privileged; single Manager user (uq_users_one_manager) |
| Staff | all rows | any | privileged |
| Volunteer | all rows | any | privileged (existing policy; route not in loan allow-list — route blocks first) |
| Donor | own only | own only | tested: 0 rows, single-record denied |
| Beneficiary | own only | own only | tested |
| Student | own only | own only | tested |

Terminology: internal `RolePartner = "Partner"` retained for DB/API
compatibility; user-facing display is "Manager"
(`RoleDisplayName`, `users.html` `<option value="Partner">Manager</option>`,
`roleDisplay()` JS). **No duplicate Manager role was created.** No code
path lets a normal role create/assign/promote to Super Admin (role
assignment restricted in user handler; loans never touch roles).

## 10. Alternate Routes / IDOR (Phase 9) — PASS (code-verified; sync NOT live-tested)

All `/loans*`, `/loan-repayments*` routes funnel through the two
services hardened here. The sync API is role-gated to
Super Admin/Admin/Staff and stamps `CreatedByID` from the session; it
cannot be used by Beneficiary/Student to reach another user's loan.
No HTMX/templ endpoint bypasses the service layer. HTML `Page`
endpoints now authenticate and pass the actor to the same scoped
service call (previously they called the service without any actor —
this was part of the unscoped-list gap).

## 11. Live PostgreSQL Test Results (Phase 10) — PASS

`PWAMS_FORCE_INTEGRATION=1`, real PostgreSQL, app user `pwams_user`
(the `.env` on this machine contains `DB_USER=pwams_db`, which does not
exist; tests were run with a `DB_USER=pwams_user` override — same
procedure as STEP 15.6.1).

```
go test -count=1 -run 'Loan' -v ./internal/services
--- PASS: TestLoanListObjectLevelAuthorization          (list/count/search/filter/pagination/nil-actor/invalid-input)
--- PASS: TestLoanRepaymentListObjectLevelAuthorization (repayment scoping + loan_id-filter IDOR)
--- PASS: TestLoanSingleRecordAuthorization             (matrix A-J, review denial)
--- PASS: TestLoanRepaymentMutationSafety               (before/after DB assertions)
--- PASS: TestLoanListFailClosedWithoutDB               (unit; nil repos)
go test -count=2 ./internal/services ./internal/handlers   -> ok / ok
go test -count=2 ./...                                     -> all ok
```

## 12. gofmt / vet / build / test — PASS

- `go build ./...` — clean
- `gofmt -l internal cmd` — clean (after formatting new/edited files)
- `go vet ./...` — clean
- `go test -count=2 ./...` — all packages ok (integration skipped
  without the env flag; with it, all PASS as above)

## 13. Race result — BLOCKED

`go test -race ./internal/services` fails to build on this machine:
`-race requires cgo`; with `CGO_ENABLED=1` the failure is
`cgo: C compiler "gcc" not found: exec: "gcc": executable file not
found in %PATH%`. There is no C toolchain installed. **Race
verification is BLOCKED, not PASS.**

## 14. Files Changed (this step)

| File | Change |
|---|---|
| `internal/services/ownership.go` | `ownershipFilter` returns `(uuid.UUID, bool)`; nil actor fails closed (corrected finding) |
| `internal/repository/loan_repository.go` | `List(query, ownerID)`; parenthesized search; `created_by_id` scope before COUNT+Find |
| `internal/repository/loan_repayment_repository.go` | `List(query, ownerID)`; `EXISTS` parent-loan owner scope before COUNT+Find |
| `internal/services/loan_service.go` | `ListLoans(query, actor)`; fail closed; person_id validation |
| `internal/services/loan_repayment_service.go` | `List(query, actor)`; fail closed; `MarkOverdue` explicit unrestricted scope (system job, commented) |
| `internal/handlers/loan_handler.go` | `Page`/`List` authenticate, pass actor; 403 mapping for `ErrRecordAccessDenied` |
| `internal/handlers/loan_repayment_handler.go` | same for repayment Page/List |
| `internal/services/person_service.go`, `student_service.go`, `donation_service.go` | fail-closed `ownershipFilter` call sites (prior STEP 15.6.1 work preserved and strengthened) |
| `internal/services/aid_request_service.go`, `internal/handlers/aid_request_handler.go` | **build-blocking pre-existing inconsistency** (repo signature took `ownerID` but service/handlers had not been migrated) completed with the same pattern |
| `internal/services/authz_fixtures_test.go` | `makeLoan`, `makeRepayment` fixtures + cleanup |
| `internal/services/authz_loan_test.go` | NEW: full integration + unit test suite for this step |

No unrelated rewrites, no generated-file corruption, no
credentials/secrets added, no `.env` committed, no IP in audit logs, no
Python, no WebSocket, no JWT localStorage.

## 15. Remaining Findings

1. **CareProvided list scoping** — `ListCareProvided` operates without
   an actor/ownership scope (existing behavior, out of this step's
   mandate) — recommended as the next hardening target.
   (`ListAidRequests` was completed here because the working tree
   contained a build-breaking half-migration.)
2. **Sync push/pull for loan entities** — privileged-role-gated and
   server-authoritative, but HTTP-level authorization was code-reviewed
   only (NOT VERIFIED live).
3. **`.env` mismatch** — `DB_USER=pwams_db` in the local `.env` does not
   match the provisioned role `pwams_user`; the app/tests cannot boot
   from `.env` alone. Operational issue, not a code defect.
4. **`RolePartner` naming** — LOW (pre-existing, documented in STEP
   15.3); display-layer mapping is in place.

## 16. Risk Severity

- Highest remaining severity: **MEDIUM** (unscoped care-provided list;
  sync HTTP verification pending).
- All previously identified HIGH loan/repayment IDOR and list-leak
  risks are **closed and live-tested**.

## 17. Release Impact

- No schema migrations. No API contract changes except: loan/repayment
  list endpoints now return only authorized rows (intended security
  fix) and may return 403 for a nil/invalid auth context. UIs rendering
  other users' loans will show fewer rows — this is the fix, not a
  regression.

## 18. Exact Next Recommended Step

**STEP 15.6.4** — harden `ListCareProvided` (DB-level actor scoping, same
pattern) and live-verify the sync push/pull authorization for loan
entities over HTTP. Then, and only then, proceed to STEP 15.7.

---

## Final Summary

- **PASS: 14** (route inventory, ownership model, matrix A-J, list
  scoping, SQL precedence, repayment authorization, mutation safety,
  privileged matrix, alternate routes, live PostgreSQL tests,
  gofmt/vet/build, unit tests, count=2 runs, diff safety)
- **FAIL: 0**
- **PARTIAL: 1** (aid-request list completed as build fix; full
  aid-request test coverage belongs to its own step)
- **BLOCKED: 1** (race detector — no gcc/cgo toolchain on this machine)
- **NOT VERIFIED: 2** (sync HTTP-level loan authorization;
  loan delete/cancel endpoints — endpoints do not exist)
- **Highest remaining severity:** MEDIUM (care-provided list scoping)
- **Exact next step:** STEP 15.6.4 — care-provided list scoping + live
  sync authorization verification, then review before STEP 15.7.

