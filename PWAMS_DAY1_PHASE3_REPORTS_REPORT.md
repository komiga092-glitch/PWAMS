# PWAMS — Day 1 / Phase 3 — Reports Functional Completion Report

Date: 2026-09-12
Scope: Reports module only. No business logic, architecture, or other modules redesigned. No fake data or fake reports.

---

## 1. Starting inventory (current source only)

| Layer | Before this phase |
|---|---|
| Routes | 4 routes: `/reports/page`, `/reports/dashboard`, `/reports/donations`, `/reports/aid-requests` |
| Handler | `report_handler.go` — 4 methods; `Page` + 3 JSON-only endpoints |
| Service | `report_service.go` — 3 methods (dashboard, donations, aid requests) |
| Repository | `report_repository.go` — 3 summary queries, counts only |
| Templates | `reports.html` (3 link cards, no data); `components/reports.templ` unused; **`reports_content` was rendered from a non-existent template** |
| JSON vs HTML | All three report endpoints returned raw JSON; browser "View Reports" landed on JSON |
| Report count | **3 areas** (Dashboard, Donations, Aid Requests), counts-only, no filtering, no pagination, no detail tables |

---

## 2. Implemented in this phase — 13 report areas (+ 1 detail view)

Every report page returns HTML inside the application layout (`base.html`), shows
KPI/totals, shows breakdown tables, and every area also has a JSON endpoint.
All routes sit behind `RequireAuth` + `RequireAnyRole(Super Admin, Admin, Partner)`
(RBAC); reports are aggregates for privileged roles, so no additional ownership
scoping applies (ownership rules govern record CRUD, not summaries).

| # | Report area | HTML page | JSON | KPIs | Breakdowns |
|---|---|---|---|---|---|
| 1 | Dashboard | `/reports/dashboard/page` | `/reports/dashboard` | Users, Persons, Students, Donors, Donations, Aid Requests, Care Provided | — |
| 2 | Users | `/reports/users/page` | `/reports/users` | Total / Active / Disabled / Locked | ByRole, ByStatus |
| 3 | Beneficiaries/Persons | `/reports/persons/page` | `/reports/persons` | Total / Active / Inactive | ByStatus, ByGender |
| 4 | Students | `/reports/students/page` | `/reports/students` | Total / Active / Inactive / Pending | ByStatus, ByGrade, ByAcademicYear |
| 5 | Donors | `/reports/donors/page` | `/reports/donors` | Total / Active / Inactive | ByType, ByStatus |
| 6 | Donations | `/reports/donations/page` | `/reports/donations` | Total count + Total amount | ByType, ByStatus (+ link to detail) |
| 7 | Aid Requests | `/reports/aid-requests/page` | `/reports/aid-requests` | Total / Pending / Approved / Rejected / Cancelled | ByType, ByPriority |
| 8 | Care Provided | `/reports/care-provided/page` | `/reports/care-provided` | Total / Completed / Pending / Cancelled | ByType, ByStatus |
| 9 | Loans | `/reports/loans/page` | `/reports/loans` | Total / Active / Pending / Approved / Completed / Total amount | ByStatus |
| 10 | Loan Repayments | `/reports/loan-repayments/page` | `/reports/loan-repayments` | Total / Paid / Pending / Overdue / Amount / Paid amount | ByStatus |
| 11 | Revenue | `/reports/revenue/page` | `/reports/revenue` | Income / Expenses / Net | Net ByCategory |
| 12 | Account Status | `/reports/account-status/page` | `/reports/account-status` | Total / Active / Disabled / Locked | ByStatus (+ ByRole in JSON) |
| 13 | Audit/System Alerts | `/reports/audit-log/page` | `/reports/audit-log` | Total entries | ByAction, ByEntity |
| + | Donation Detail | `/reports/donation-detail/page` | `/reports/donation-detail` | Record table | status / search / date-range filters + pagination |

Filtering + pagination: the Donation Detail view supports `status`, `search`,
`from_date`, `to_date`, `page`, `page_size` (default 20, max 100) with
Previous/Next pagination controls. The other 13 areas are aggregate summaries
(counts and totals) by design; their row-level "detail" pattern is established
by the Donation Detail view for future extension.

---

## 3. Dashboard link fix

`web/templates/dashboard.html` "View Reports" button previously targeted
`/reports/dashboard` (raw JSON). It now targets `/reports/dashboard/page`
(HTML report page), role-gated `data-roles="Super Admin,Admin,Partner"`.
The HTML sidebar (`layouts/header.html`) and the PWA sidebar
(`components/sidebar.templ`) already pointed at `/reports/dashboard/page`.

## 4. Files changed

| File | Change |
|---|---|
| `internal/models/report.go` | 14 typed report structs + `ReportFilter` (filtering/pagination params) |
| `internal/repository/report_repository.go` | Rebuilt cleanly: GROUP-BY helpers (typed `GroupCount`/`GroupAmount` + `Scan`, matching the codebase `CountByRole` pattern; no unsupported `.Rows()`), 13 summary queries + `GetDonationDetail` (parameterized WHERE, count + paged SELECT) |
| `internal/services/report_service.go` | 11 new delegate methods (users, persons, students, donors, care, loans, repayments, revenue, account status, audit log, donation detail) |
| `internal/handlers/report_handler.go` | Rebuilt cleanly: shared `parseReportFilter`/`writeReportList`/`renderReport`/`renderReportError`, landing `Page`, 13 page methods + `DonationDetailPage`, 14 JSON methods |
| `internal/routes/report_routes.go` | 14 HTML page routes + 14 JSON routes, all inside the RBAC-protected group |
| `web/templates/reports.html` | Rewritten: reports landing (13 quick cards) + System Summary KPI panel |
| `web/templates/report_detail.html` | New: 14 page blocks (`report_dashboard_content` ... `report_donation_detail_content`) with KPI cards, breakdown tables, filter form, paginated table |
| `web/templates/layouts/base.html` | Dispatch chain extended with the 14 report template names |
| `web/templates/dashboard.html` | "View Reports" link fixed to the HTML page |
| `internal/handlers/auth_handler_test.go` | Restored `TestLogoutRedirectsBrowserRequestsAndClearsCookie`; added `TestReportTemplatesParse` and `TestReportBlocksRender` (16 sub-tests covering parse + render of every report block) |

Notes: `router.SetFuncMap` (`role_display`, `add`, `sub`, `isMap`) already existed
and is now actually used by the report templates. The stray unused artifact
`web/templates/report_page_content.html` (not in `LoadHTMLFiles`, not referenced)
was left in place untouched.
## 5. Verification results (all executed this session)

| Command | Result |
|---|---|
| `go fmt ./...` | PASS (exit 0) |
| `go test ./...` | PASS (exit 0) — handlers, middleware, models, services, utils all ok; includes the 16 new template sub-tests (parse + render of every report block, incl. gt/lt/add/sub usage) |
| `go vet ./...` | PASS (exit 0) |
| `go build ./...` | PASS (exit 0) |
| `npm run build` | PASS (exit 0) — tsc && tsc -p tsconfig.offline.json |

Template correctness is enforced at test level because HTML templates are parsed
at server start (`LoadHTMLFiles`), not at compile time; `TestReportTemplatesParse`
parses the exact files the server loads and fails on syntax or unknown functions.

## 6. Remaining blockers / not done

1. Row-level detail views for the other 12 areas — not started. The Donation
   Detail view establishes the filter+pagination pattern; extending it to other
   areas is future work (same pattern, no redesign).
2. DB-backed integration tests for report endpoints — `test_db_helper_test.go`
   requires a live PostgreSQL; no database is available in this environment, so
   endpoint-level integration tests are deferred. Template/service layers are
   covered by the unit/template tests above.
3. CSV/PDF export — out of scope for this phase (not requested).
4. Ownership-scoped report variants — not applicable: report routes are
   privileged-only; ownership rules apply to record CRUD, not aggregates.

Phase 4 not started.