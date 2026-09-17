# PWAMS — DAY 1 / PHASE 5 — AUDIT + CSP FINALIZATION REPORT

Date: 2026-09-13
Scope: Audit hardening verification, behavioral audit test coverage, and strict-CSP
inline-JavaScript remediation.

---

## 1. Audit requirements — verification results

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | No IP address in AuditLog | PASS | `internal/models/audit_log.go` has no IP field (comment: "IPAddress deliberately omitted… PII"). `database/migrate.go` idempotently **drops** a legacy `audit_logs.ip_address` column so deployed DBs converge. |
| 2 | No secrets in audit rows | PASS | `AuditLogService.Create(userID, action, entity, entityID, details)` accepts no password/OTP/token/credential fields. `AuditLog` stores only `user_id`, `action`, `entity`, `entity_id`, `details`, `old_value`, `new_value`, `request_id`. Grep of all 32 audit write call sites: no secret values are passed. Password changes audit `"PASSWORD_CHANGE"` with text "User changed own password" — never the password itself. |
| 3 | Audit authorization-sensitive mutations | PASS | 32 write sites across user, person, student, donor, donation, aid request, care provided, loan, loan repayment, revenue, file upload, message, notification, auth, activation, admin-deletion handlers. |
| 4 | Authorization/business validation BEFORE audit write | PASS | Verified at every handler: RBAC middleware → service validation (returns error before mutation) → **then** `auditLogService.Create` on success only. On any service error the handler returns before the audit call. |
| 5 | Audit failure must not bypass authorization | PASS | Authorization is enforced by middleware/service and is completely independent of the audit write. The audit call happens after the mutation; a failing audit cannot un-authorize anything. |
| 6 | Fail-open vs fail-closed decision | **FAIL-OPEN (documented)** | Every audit write is fire-and-forget (`_ = h.auditLogService.Create(...)`) or checks-and-continues (`user_handler.go`: "Audit logging failure must not fail the user creation"). Rationale: audit is a non-critical side effect; mutations must never be blocked or rolled back by audit infrastructure. This is an **honest** choice: the mutation is not blocked even if compliance data is lost — it is NOT claimed to be fail-closed. |
| 7 | No behavioral coverage claim without audit-row assertions | PASS | All new tests assert real `audit_logs` rows via `countAuditRows(...)` (see §2). |
| 8 | Behavioral tests for high-risk mutations | PASS | 13 tests, all executed against live PostgreSQL (§2). |
| 9 | Audit list authorization + IDOR | PASS | `audit_log_routes.go` restricts `/audit-logs*` to `RoleSuperAdmin, RoleAdmin` via `RequireAnyRole`. `List` paginates (page/page_size clamped 1–100) and supports optional action/entity/user_id filters (privileged review). `GetByID` validates UUID and returns 404 on `ErrAuditLogNotFound`. No ownership escalation path exists for non-privileged roles. |

## 2. Behavioral audit tests (new — `internal/services/audit_behavioral_test.go`)

All 13 tests drive the **real handler → service → repository → PostgreSQL** path in gin
TestMode (with `current_user` injected exactly like the auth middleware) and then assert
audit rows. They run whenever `PWAMS_FORCE_INTEGRATION=1` and a reachable test DB is
configured; otherwise they **skip** (never silently pass).

| Test | Asserts |
|---|---|
| TestAuditWrittenForUserCreation | users/CREATE row ≥ 1 |
| TestNoAuditForForbiddenSuperAdminCreation | 403 **and** users/CREATE rows = 0 (authorization precedes audit) |
| TestAuditWrittenForPersonCreation | persons/CREATE row ≥ 1 |
| TestNoAuditForObjectLevelAuthorizationDenial | cross-user person UPDATE refused **and** persons/UPDATE rows = 0 |
| TestAuditWrittenForStudentCreation | students/CREATE row ≥ 1 |
| TestAuditWrittenForDonorCreation | donors/CREATE row ≥ 1 |
| TestAuditWrittenForDonationCreation | donations/CREATE row ≥ 1 |
| TestAuditWrittenForAidRequestCreation | aid_requests/CREATE row ≥ 1 |
| TestAuditWrittenForCareProvidedCreation | care_provided/CREATE row ≥ 1 |
| TestAuditWrittenForLoanCreation | loans/CREATE row ≥ 1 |
| TestAuditWrittenForLoanRepaymentCreation | loan_repayments/CREATE row ≥ 1 |
| TestAuditWrittenForRevenueCreation | revenue_records/CREATE row ≥ 1 |
| TestAuditWrittenForFileDeletion | file_uploads/DELETE row ≥ 1 |

Infrastructure note: `AcquireTestDB` (test DB helper) now also calls
`database.SeedDefaultRoles` after `Migrate`, so the whole integration suite
(authz_*, audit_behavioral) runs against a fresh DB out of the box (idempotent
`FirstOrCreate`).

---

## 3. CSP — strict policy blocked inline JavaScript

The application serves `Content-Security-Policy: default-src 'self'; script-src 'self';
style-src 'self' 'unsafe-inline'; …` (`internal/middleware/security.go`). Under
`script-src 'self'` **both** inline event-handler attributes (`onclick=`, `onsubmit=`, …)
and inline `<script>` blocks are blocked. A full scan found:

- **7 inline `<script>` blocks**: `users.html`, `care_provided.html`,
  `audit_logs.html`, `forgot_password.html`, `reset_password.html`,
  `verify_reset_otp.html`, `loan_repayments.html`
- **~45 inline handler attributes** across 7 templates (users, care_provided,
  aid_requests, donations, loans, loan_repayments, revenue; plus a previously
  broken multi-line `onsubmit` in care_provided.html)

### Fixes (no CSP weakening)

1. **External page scripts** — each inline block was moved verbatim to
   `web/static/js/pages/*.js` (`users.js`, `care_provided.js`, `audit_logs.js`,
   `forgot_password.js`, `reset_password.js`, `verify_reset_otp.js`,
   `loan_repayments.js`) and referenced via `<script src="/static/js/pages/…">`.
   One latent JS syntax error (`const operation: "CREATE" | "UPDATE" = …`, a TS
   annotation in plain JS) was fixed to `const operation = …`.
2. **data-csp-action attributes** — every inline handler was replaced with a
   `data-csp-action` attribute (plus `data-csp-id/status/name/page` argument
   attributes where the old attribute passed values); runtime-generated rows in
   `users.js` / `care_provided.js` template literals were converted the same way.
3. **Central dispatcher** — new `web/static/js/csp-delegator.js` (loaded once from
   `layouts/base.html` after `app.js`) listens for `click`/`submit` and dispatches
   `data-csp-action` to the existing external page functions. Forms keep their
   original handlers (`submitDonation(event)`, `submitRepaymentAction(event, form,
   "pay")`, …) and `care:search` replicates the old `return false` guard.
4. **Regression guard** — new Go test
   `TestTemplatesContainNoInlineJavaScript` (`internal/middleware/security_test.go`)
   scans all 19 interactive templates and the base layout for inline handler
   attributes and inline `<script>` blocks, and asserts `csp-delegator.js` exists
   and dispatches `data-csp-action`.

Final state: **0 inline attributes, 0 inline `<script>` blocks** in
`web/templates/**`; all page JS files pass `node --check`.

## 4. Verification results (all executed this session)

| Command | Result |
|---|---|
| `go fmt ./...` | PASS (exit 0) |
| `go test ./...` | PASS (exit 0) — full suite incl. 13 live-DB behavioral audit tests + CSP regression test |
| `go vet ./...` | PASS (exit 0) |
| `go build ./...` | PASS (exit 0) |
| `npm run build` | PASS (exit 0) |

## 5. Files changed / added

**Changed:**
- `internal/services/test_db_helper_test.go` — seed default roles after migrate
- `internal/middleware/security_test.go` — static CSP regression test
- `web/templates/layouts/base.html` — load `csp-delegator.js`
- `web/templates/{users,care_provided,aid_requests,donations,loans,loan_repayments,revenue,audit_logs,forgot_password,reset_password,verify_reset_otp}.html`

**Added:**
- `internal/services/audit_behavioral_test.go` — 13 behavioral audit tests
- `web/static/js/csp-delegator.js`
- `web/static/js/pages/{users,care_provided,audit_logs,forgot_password,reset_password,verify_reset_otp,loan_repayments}.js`

## 6. Remaining blockers

None blocking this phase. Honest limitations:

1. **Audit is fail-open by design** — a failing audit write does not fail the
   mutation; this is documented, not hidden (see §1.6).
2. The behavioral audit tests require a live PostgreSQL (they skip otherwise);
   CI provides one via the `postgres` service in `.github/workflows/ci-cd.yml`.
3. The old `.templ` component files (`web/templates/components/*.templ`) are
   unused legacy; untouched. They contain no inline handlers either.
4. Browser-level behavior after the data-attribute migration is covered by the
   static regression test and `node --check`, not by an end-to-end browser suite
   (no browser test harness exists in the repo).

Phase 6 not started.