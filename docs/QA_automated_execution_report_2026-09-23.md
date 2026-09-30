# PWAMS — Automated Execution Report (Interim)

**Date:** 2026-09-23
**Executor:** Cline (automated coding agent) — *not* a human manual-QA pass; see honesty statement below
**Target checked:** `http://localhost:8081` (local dev instance built from current source, Postgres `localhost:5432`)
**Status of the 232-TC workbook:** NOT RECEIVED — the Excel attachment did not reach the workspace. No `.xlsx/.xls/.csv` exists anywhere in `E:\PWAMS`. Columns 1–5 (Actual Result / Status / Tested By / Date Tested / Remarks) therefore **cannot be filled yet**.

---

## Honesty statement

1. Every result below was genuinely executed and captured live on the date shown. Nothing is extrapolated.
2. Rows marked "Blocked" were **not** executed and are not marked Pass.
3. For any workbook row later attributed to this work, "Tested By" must read **"Cline (automated)"**, not a human name. Human sign-off rows require a human tester.
4. `localhost:8081` is a **local dev instance**. It has not been confirmed to be the "live or staging" environment the report covers.

---

## A. Automated Go test suite — `go test ./... -count=1 -v`

**Result: 8/8 packages OK — 626 passed, 0 failed, 63 skipped (by design).**

| Package | Result | Duration |
|---|---|---|
| `internal/database` | ok | 0.282s |
| `internal/handlers` | ok | 0.298s |
| `internal/middleware` | ok | 1.228s |
| `internal/models` | ok | 0.833s |
| `internal/repository` | ok | 0.834s |
| `internal/routes` | ok | 3.263s |
| `internal/services` | ok | 0.337s |
| `internal/utils` | ok | 0.731s |

- **Skips (63):** the live-DB integration suite (`audit_behavioral_test.go`, `migrations_schema_integration_test.go`, `tenant_scope_integration_test.go`, `migrator_integration_test.go`, and friends) is opt-in via `PWAMS_FORCE_INTEGRATION=1` per `internal/services/test_db_helper_test.go:63-70`. Enabling it will run migrations + seed + fixtures against the configured DB — available on request.
- **Module coverage mapping (repo test files → manual-test modules):**
  - *Auth:* `auth_handler_test.go`, `routes/auth_flow_blackbox_test.go`, `services/user_service_test.go`
  - *Security/RBAC:* `middleware/rbac_middleware_test.go`, `middleware/security_test.go`, `middleware/rate_limit_test.go`, `routes/csp_regression_test.go`, `routes/rbac_ui_visibility_test.go`, `services/authz_*_test.go`, `services/ownership_test.go`, `services/user_superadmin_guard_test.go`
  - *Donations / Loans:* `services/authz_donation_test.go`, `services/authz_loan_test.go`, `services/loan_service_test.go`
  - *Students / Donors:* `services/authz_student_test.go`, `services/authz_person_test.go`, `services/authz_object_level_donor_test.go`
  - *Reports:* `routes/report_blackbox_test.go`, `routes/report_registry_test.go`
  - *Files:* `handlers/file_upload_validation_test.go`, `services/file_upload_service_test.go`
  - *Sync (offline PWA):* `services/sync_service_test.go`, `repository/sync_pagination_test.go`
  - *Audit:* `services/audit_behavioral_test.go` (live-DB — currently skipped)
  - *OTP:* `utils/otp_test.go`
  - *Migrations/schema:* `database/*_integration_test.go` (live-DB — currently skipped)

## B. HTTP-level smoke checks against `http://localhost:8081`

| # | Check | Actual Result | Verdict |
|---|---|---|---|
| B1 | `GET /login` renders login page | `200 OK`, HTML login form (login + password fields) | **Pass** |
| B2 | Security headers | CSP `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; frame-ancestors 'none'; form-action 'self'; object-src 'none'` + `X-Frame-Options: DENY`, HSTS (2y, preload), `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, COOP/COEP/CORP, `Permissions-Policy: camera=(), microphone=(), geolocation=()`, CSRF token in `X-Csrf-Token` header + `pwams_csrf` cookie (`SameSite=Strict`) | **Pass** |
| B3 | Unauthenticated access to protected page | `GET /dashboard` → `303 See Other` → `Location: /login` | **Pass** |
| B4 | Invalid-credentials login (valid CSRF supplied) | `200` + re-rendered login page showing **"Invalid username/email or password."** — generic, no user-enumeration leak (verified for a nonexistent user; matches `internal/handlers/auth_handler.go:137`) | **Pass** |
| B5 | Login rate limiting | Subsequent failed attempts → **`429 Too Many Requests`** after ~5 attempts in the window. Middleware enforces **5 per 15 min per IP** (`internal/middleware/rate_limit.go:87,93`) and returns JSON `{"success":false,"message":"Too many requests. Please try again later."}` + `Retry-After: 60` (body per code + `rate_limit_test.go`; live capture confirmed the 429 status) | **Pass** |
| B6 | Unknown route handling | `GET /definitely-not-a-route-qa` → `404` | **Pass** |
| B7 | Static assets | `/static/css/app.css` → 200 (35,298 B); `/static/js/app.js` → 200 (9,123 B); `/favicon.ico` → 200 (1,504 B) | **Pass** |

**Remark (B5):** earlier `status=0` readings were a client-side script issue (session variables not persisting between separate shell invocations); the numbers above are from a clean single-shell re-run.

---

## C. Not executed / Blocked

| Category | Why blocked |
|---|---|
| The 232-TC workbook rows (columns 1–5) | Workbook attachment not received; cannot edit what is not here |
| Browser/visual TCs (layout, responsiveness, print/export UI, HTMX polish, Sinhala font rendering) | Require a human in a real browser |
| Authenticated CRUD walkthroughs for all 21 modules | Need per-role test accounts for the target environment |
| OTP + password-reset email flows | SMTP not configured here (`SMTP_HOST` empty in `.env.example`; actual `.env` not inspected) — needs an SMTP sandbox |
| 63 live-DB integration tests | Opt-in via `PWAMS_FORCE_INTEGRATION=1`; will migrate/seed the configured DB — run on request |
| Sending the completed workbook back | Not possible from this environment; deliverables are placed in `E:\PWAMS` for you to send |

## D. What is needed to fill the workbook

1. **Re-attach the workbook** (or drop the file into `E:\PWAMS`).
2. **Confirm the target environment** — is `localhost:8081` the agreed environment, or a separate staging URL?
3. **Per-role test accounts** (or consent to run the live-DB integration suite locally).
4. **SMTP sandbox details** if OTP/reset email TCs are in scope.

## Artifacts

- `qa_go_test_stdout.txt` — full verbose suite output (81 KB)
- `qa_pkg_results.txt`, `qa_skip_count.txt`, `qa_skip_reasons.txt` — extracted summaries
- `qa_go_test_pid.txt` — background-run bookkeeping (safe to delete)

