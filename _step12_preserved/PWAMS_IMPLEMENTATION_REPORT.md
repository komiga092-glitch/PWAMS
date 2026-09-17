# PWAMS Implementation Report

People Welfare Association Management System
Date: 2026-08-29 · Branch: `main`

---

## 1. Executive Summary

PWAMS is an existing NGO management system built with Go (Gin + GORM), server-side
HTML with HTMX/templ, a PWA/offline-first client (Service Worker + IndexedDB), and
PostgreSQL. This work continues an in-progress hardening effort. The prior session
completed the **account creation + activation** foundation (no creator-set password,
`Pending` status, token-based activation). This session added the **granular
permission system, server-side permission enforcement, a permission-management API,
a supervised Super Admin deactivation workflow, CHANGE_ROLE / deactivation /
permission audit logging, permission-aware sidebar + navigation, and matching
tests**. No working systems were rewritten; the existing role-based middleware was
preserved and extended.

PWAMS remains an NGO management system. Loan/repayment and financial tracking remain
NGO-oriented (beneficiary micro-credit); no ledger/accounting system was introduced.

---

## 2. Features Implemented

- Granular permissions following `resource.action` convention (`users.view`,
  `donor.create`, `aid.approve`, `reports.view`, …).
- Permission database model: `permissions`, `role_permissions`, `user_permissions`.
- `PermissionService`: default seeding, role-based defaults, per-user overrides,
  escalation guards.
- Permission middleware: `LoadPermissions`, `RequirePermission`,
  `RequireAllPermissions` — server-side, per-request cached.
- Permission-management API: `/permissions/*` (catalog, role matrix, user overrides)
  with privilege-escalation guards.
- Super Admin deactivation workflow: request → notify target → accept/reject, with
  `pending/accepted/rejected/expired` states; stale requests auto-expire.
- Completed account hierarchy: Super Admin → (SA/Admin/Staff/Volunteer),
  Admin → (Staff/Volunteer), Staff & Volunteer → (Donor/Beneficiary).
- Audit logging: `CHANGE_ROLE`, `REQUEST_DEACTIVATION`, `ACCEPT_DEACTIVATION`,
  `REJECT_DEACTIVATION`, `GRANT_PERMISSION`, `REVOKE_PERMISSION`,
  `CHANGE_PERMISSION` (in addition to existing login/user/create/update/delete logs).
- Permission-aware UI: `/auth/me` returns the permission set; sidebar/nav supports
  `data-permissions`.
- Self-deactivation guard: a Super Admin cannot deactivate their own account.

---

## 3. Files Changed

**Models**
- `internal/models/permission.go` (new) — Permission, RolePermission, UserPermission.
- `internal/models/deactivation_request.go` (new) — request entity + payloads.
- `internal/models/user.go` — `PasswordHash` nullable (pending accounts).
- `internal/models/role.go` — description/soft-delete support.
- `internal/models/create_user_request.go` — removed creator-set password.

**Repositories**
- `internal/repository/permission_repository.go` (new).
- `internal/repository/deactivation_request_repository.go` (new).
- `internal/repository/role_repository.go` — added `List()`.
- `internal/repository/account_activation_repository.go` — token-hash activation.

**Services**
- `internal/services/permission_service.go` (new).
- `internal/services/permission_definitions.go` (new) — canonical catalog.
- `internal/services/permission_role_defaults.go` (new) — default role matrix.
- `internal/services/deactivation_request_service.go` (new).
- `internal/services/account_activation_service.go` — activation audit.
- `internal/services/user_service.go` — pending status on create.

**Handlers**
- `internal/handlers/permission_handler.go` (new).
- `internal/handlers/deactivation_request_handler.go` (new).
- `internal/handlers/user_handler.go` — hierarchy, CHANGE_ROLE audit,
  self-deactivation guard, escalation guards.
- `internal/handlers/account_activation_handler.go`.

**Middleware**
- `internal/middleware/permission.go` (new) — permission loading + checks.

**Routes**
- `internal/routes/permission_routes.go` (new).
- `internal/routes/deactivation_request_routes.go` (new).
- `internal/routes/auth_routes.go` — `/auth/me` returns permissions; protected group
  loads permissions.
- `internal/routes/user_routes.go`, `internal/routes/donor_routes.go` — granular
  permission gates.

## 4. Database Changes

- `users.password_hash` is now nullable (pending accounts have no password).
- `users.status` default remains `'Active'`; app-created accounts are set to
  `'Pending'` by the service (seeded Super Admin stays `'Active'`).

## 5. New Migrations

- `migrations/000002_permissions_deactivation.up.sql`
  - `permissions`, `role_permissions`, `user_permissions`,
    `deactivation_requests` tables.
  - Drops NOT NULL on `users.password_hash`.
- `migrations/000002_permissions_deactivation.down.sql`
  - Drops the new tables; restores `password_hash` NOT NULL.

## 6. New Routes

- `GET /permissions` — permission catalog.
- `GET/PUT /permissions/roles` and `/permissions/roles/:name` — role matrix.
- `GET/PUT /permissions/users/:id` — user permission overrides.
- `GET/POST /super-admins/deactivations`, `POST /:id/accept`, `POST /:id/reject`.
- `/auth/me` now returns `permissions[]`.
- `/users/*` and `/donors/*` mutation endpoints gained granular permission gates.

## 7. New Permissions

Catalog (subset): `users.*`, `super_admin.*`, `admin.*`, `staff.*`,
`volunteer.*`, `donor.*`, `beneficiary.*`, `person.*`, `student.*`,
`donation.*`, `aid.*` (`/approve`), `care.*`, `loan.*` (`/approve`),
`repayment.*`, `revenue.*`, `file.*`, `message.*`, `notification.*`,
`reports.view`, `audit_logs.view/export`, `permission_management.view/edit`,
`account_management.view`.

Default matrix (abridged): Super Admin = all; Admin = management + approvals +
reports + audit + permission mgmt; Staff = register/edit donor/beneficiary/
person/student/donation/aid/care/loan/repayment (no delete) + `users.create`;
Volunteer = read + create on operational entities, no delete;
Donor/Beneficiary = own-data read only (no `users.view`, preventing directory leak).

## 8. New Middleware

- `middleware.LoadPermissions` — caches a user's effective permission set per request.
- `middleware.RequirePermission` — 403 without the permission.
- `middleware.RequireAllPermissions` — 403 unless all are held.

## 9. Security Fixes / Additions

- Server-side permission enforcement (frontend hiding is not security).
- Role-escalation guards in Create/Update/UpdateStatus/permission management.
- Super Admin cannot self-deactivate.
- Donor/Beneficiary no longer get `users.view` (prevents directory leakage).
- Super Admin deactivation requires an approval workflow, not a direct call.
- Permission and role changes are audited.

## 10. Account Lifecycle Changes

New accounts are created `Pending` with no password; the invited user sets their own
password via a secure, single-use, expiring activation token; the account becomes
`Active`. Pending/Disabled/Locked users cannot log in (existing auth logic).

## 11. Dashboard Changes

No dashboard redesign. The permission-aware sidebar/nav hides links the user cannot
access.

## 12. Donation / 13. Aid & Care / 14. Loan & Repayment

Added the corresponding `donation.*`, `aid.*`, `care.*`, `loan.*`,
`repayment.*` permission gates and defaults. Financial calculations remain
centralized in the existing loan/repayment services.

## 15. Notification Changes

`notification.view`/`create` permissions added; nav hidden without `notification.view`.

## 16. Audit Logging Changes

- `CHANGE_ROLE` recorded with old→new role.
- `REQUEST_DEACTIVATION`, `ACCEPT_DEACTIVATION`, `REJECT_DEACTIVATION`,
  `EXPIRE_DEACTIVATION`.
- `GRANT_PERMISSION`, `REVOKE_PERMISSION`, `CHANGE_PERMISSION`.

## 17. Offline / PWA Changes

No offline-first architecture changes; only the sidebar JS/HTML support
`data-permissions`. PII handling and the sync queue are unchanged.

## 18. Tests Added / Updated

- `internal/handlers/user_role_permission_test.go` — account-create hierarchy
  matrix; Staff/Volunteer cannot create elevated accounts.
- `internal/middleware/permission_test.go` — permission allow/deny, all-permission
  checks, unauthenticated denial, context checks, loader nil/anon handling.

## 19. Commands Executed / 20. Test Results

- `go build ./...`, `go vet ./...`, `go test ./...` (fresh run after
  `go clean -testcache`), `go fmt ./...`, `npm run build`.
- All packages pass; build and vet are clean.

## 21. Known Limitations

- Offline sync does not carry permission state (permissions remain
  server-authoritative; writes verify on push, so this is intentional rather
  than a functional gap).
- Donor/Beneficiary self-service dashboards display their own data via the

---

# Final Professionalization Report — 2026-09-03

Phases delivered on top of the report above. No rebuild, no stack changes.

## A. Executive Summary

- **i18n (Phase 1)**: Complete en/ta/si coverage. Reusable header language
  selector on every authenticated page (no-JS-safe form posts to
  `POST /profile/language`), single shared server-side preference, HTMX
  `HX-Refresh` support, Partner→Manager display mapping via `DisplayRole`/`tRole`.
- **User Management (Phase 2)**: Consolidated sidebar — role account pages
  (Admins/Partners/Staff/Volunteers management links) removed; one **Users**
  entry. Stats cards (`GET /users/stats`), status filter, clear filters,
  card-based View modal, Personal Information fields in the single Add/User
  form, modal-based status change (no `prompt()`).
- **System Alerts (Phase 3)**: Severity enum incl. CRITICAL, dedup
  (occurrence_count/first/last), migration `000005` + AutoMigrate-safe
  backfill/dedup-index hardening moved after schema creation.
- **Super Admin owner dashboard, fail-closed RBAC audit (5 verified
  fail-closed sites), reporting, card-based detail views** delivered in prior
  steps of this engagement (see report tables in the chat response).

## B. Files Changed (this phase)

| File | Change |
|---|---|
| `internal/database/migrate.go` | System-alert hardening block moved AFTER AutoMigrate transaction |
| `internal/repository/user_repository.go` | `CountByStatuses()` grouped aggregation |
| `internal/services/user_service.go` | `UserStats()` |
| `internal/handlers/user_handler.go` | `Stats()`, form-encoded `UpdateLanguage` + `redirectAfterLanguageChange` |
| `internal/routes/user_routes.go` | `GET /users/stats` behind `users.view` |
| `internal/i18n/{english,tamil,sinhala}.go` | `users.*`, `common.*` selector/UM keys (all 3 langs) |
| `web/templates/layouts/base.html` | Reusable language selector (no-JS form buttons) |
| `web/templates/layouts/header.html` | Removed duplicate Admin-management sidebar block |
| `web/templates/users.html` | Full-name/phone fields, 8-col table, stats, status modal, detail cards, localized JS |

## C. Build Validation

| Command | Result |
|---|---|
| `go build ./...` | PASS (exit 0) |
| `go vet ./...` | PASS (exit 0) |
| `go test ./internal/... ./web/...` | PASS (all ok) |
| `gofmt -l` | Clean (touched files) |
| `npm run build` (tsc ×2) | PASS (exit 0) |
| `templ generate` (v0.3.1020) | PASS, generated output consistent |

## D. Fail-Closed Verification

| Site | Behaviour |
|---|---|
| `middleware/permission.go:29` (LoadPermissions) | nil service → skip cache; RequirePermission guard denies |
| `middleware/permission.go:134` (hasPermission) | nil service → `errPermissionCheckFailed` → 500 deny |
| `routes/permission_routes.go:28` | module NOT registered |
| `routes/system_alert_routes.go:26` | module NOT registered |
| `routes/system_settings_routes.go:26` | module NOT registered |

## E. Remaining Work / TODO

- Guest/login locale resolution uses safe default (English) + cookie-free
  strategy; Accept-Language negotiation intentionally not added.
- Role-specific dynamic sections in the Add User form (donor/student/
  beneficiary/manager extras) intentionally deferred: User Management owns
  identity/role/status only; domain modules own their records (spec-compliant).
- Report export remains gated by existing `report.export` / `report.pdf`
  permissions; export surface follows prior-phase implementation.

  existing dashboard/profile endpoints; deep per-module self-service views
  remain role-gated and server-owned.

## 22. Remaining Technical Debt

- Optionally cache the permission set in the offline IndexedDB model to shave
  a round trip on cold page loads; server remains authoritative either way.
- The permission-management UI currently edits role-default matrices; the
  per-user override endpoints (already implemented) are exposed via the API and
  could be surfaced in a future screen if desired.

## 23. Deployment Notes

- Run the forward migration `000002_permissions_deactivation.up.sql`.
- On startup the server auto-migrates (`AutoMigrate`) and seeds roles +
  permissions idempotently.
- Roles must exist before permission seeding (handled automatically in main.go).
- No secrets are hard-coded; `.env`/secret management unchanged.

---

### Final Status

- Completed: permission system + enforcement across **all** module route
  groups, permission-management API **and** a dedicated HTML management screen
  (`/permissions/page`), Super Admin deactivation approval workflow, expanded
  audit logging, completed account hierarchy + activation, permission-aware
  sidebar.
- Granular permission gates: /users, /donors (prior session) **plus** persons,
  students, donations, aid-requests, care-provided, loans, loan-repayments,
  revenue, files, messages, notifications, reports, audit-logs (this session).
  Role gates remain layered as defense-in-depth; `RequirePermission` now
  returns 403 for missing granular permissions across every module.
- Migrations: `000002_permissions_deactivation` (up + down).
- Tests: account hierarchy, permission middleware module gates, activation;
  all green.
- Security: server-side authz, escalation guards, self-deactivation protection,
  activation token lifecycle, audit integrity in place.
- Deployment readiness: `go build`, `go vet`, `go fmt`, `go test ./...` and the
  TypeScript frontend build all pass; forward migration required.


**Web / Frontend**
- `web/static/ts/app.ts` (and compiled `app.js`, `app.js.map`).
- `web/templates/layouts/header.html` — `data-permissions` on nav.
- `web/templates/permissions.html` + `web/static/js/permissions.js` — new
  role-permission management screen (this session).
- `web/templates/layouts/base.html` — added `permissions_content` case.

**App wiring**
- `cmd/server/main.go` — wired permission + deactivation services/handlers/routes,
  permission seeding, deactivation expiry job, and passed `permissionService`
  into every module route group (this session).
- `internal/database/migrate.go` — registered new models for AutoMigrate.
- Tests: `internal/handlers/user_role_permission_test.go`,
  `internal/middleware/permission_test.go` (module-gate coverage added this
  session).
