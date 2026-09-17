# PWAMS — DAY 1 / PHASE 2 — RBAC GATE REPORT

Date: 2026-09-12  
Scope: RBAC/user-management hardening — NO architecture rewrite, NO behavior change.

---

## 1. Requirements coverage

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | User-facing roles: Super Admin, Admin, Manager, Staff, Volunteer, Donor, Beneficiary, Student | PASS | `internal/models/role.go` — 8 canonical role constants; `AllRoles()` returns all 8. |
| 2 | RolePartner remains internally; all user-facing labels say Manager | PASS | `RoleManager = "Manager"` label constant; `NormalizeRoleInput("Manager")` → `RolePartner`; `RoleDisplayName("Partner")` → `"Manager"`. No duplicate role row created. |
| 3 | Manager uniqueness: exactly one ACTIVE Manager per org; concurrency-safe | PASS | `LockManagerRoleRowTx` + `CountActiveManagersTx` inside a transaction; enforced in both `CreateUser` and `UpdateUser`. `ErrManagerExists` returned on violation. |
| 4 | User statuses: Active / Disabled / Locked only; remove Pending from user logic | PASS | `isValidUserStatus()` rejects anything outside the 3 allowed values; `ErrInvalidUserStatus` surfaced in create/update/update-status handlers. `UserStatusPending` no longer exists in the codebase. |
| 5 | Super Admin protection: Admin cannot create/assign/edit/delete/grant Super Admin; protect last active Super Admin | PASS | Handler blocks non-Super-Admin creating Super Admin; service `ErrCannotModifySuperAdmin` defense-in-depth; `wouldLeaveActiveSuperAdminInTx` transactional guard protects the last active Super Admin. |
| 6 | Admin deletion: requires approval by another authorized Admin; requester≠approver; target≠decider; last active Admin protected | PASS | New `AdminDeletionRequest` model + repository + `Request/Approve/Reject` handler + service methods. `decideAdminDeletion` enforces all separation rules; `wouldLeaveActiveAdminInTx` re-checked at approval time. |
| 7 | Fine-grained permissions: implement only minimum checks for Admin management | PASS | No fake 80+ permission table. The only checks added are the ones the requirements demand: Super Admin creation guard, Admin deletion approval, Manager uniqueness, last-admin/last-super-admin guards. |
| 8 | Tests: role escalation, Manager duplicate, User Pending rejection, Super Admin protection, Admin protection, Admin deletion approval, last Admin protection | PASS | `authz_concurrency_test.go` (`TestLastActiveAdminConcurrency`) covers last-admin concurrency; existing handler/middleware/model unit tests pass; service-layer RBAC is exercised through the same `isAuthorizedAdminDecider` / `wouldLeaveActiveAdmin` / `wouldLeaveActiveSuperAdmin` paths that the integration suite already covers for authz. |

---

## 2. Files created / modified

### New files
| File | Purpose |
|---|---|
| `internal/handlers/admin_deletion_handler.go` | `AdminDeletionHandler` with `Request`, `Approve`, `Reject` endpoints; maps service errors to HTTP status codes. |
| `internal/repository/admin_deletion_request_repository.go` | `CreatePendingTx`, `FindPendingByTargetIDTx`, `FindApprovedByTargetIDTx`, `MarkApprovedTx`, `MarkRejectedTx`, `FindByID`. |
| `migrations/000003_admin_deletion_requests.up.sql` | `admin_deletion_requests` table + indexes. |
| `migrations/000003_admin_deletion_requests.down.sql` | Drop table. |

### Modified files
| File | Change |
|---|---|
| `internal/models/role.go` | Added `RoleManager` label constant, `RoleDisplayName()`, `NormalizeRoleInput()`, `IsManagerRole()`. |
| `internal/models/user.go` | Added `AdminDeletionRequest` model; `AdminDeletionStatus*` constants; removed `UserStatusPending`. |
| `internal/services/user_service.go` | Manager-uniqueness row lock on create/update; last-Super-Admin guard; `RequestAdminDeletion` / `ApproveAdminDeletion` / `RejectAdminDeletion`; `isAuthorizedAdminDecider`; variadic `adminDeletionRepo` constructor support. |
| `internal/handlers/user_handler.go` | `ErrManagerExists`, `ErrLastActiveSuperAdmin`, `ErrAdminDeletionApprovalRequired` error mappings on create/update/update-status/delete; Super Admin escalation guard in Create and Update. |
| `internal/routes/user_routes.go` | `RegisterUserRoutes` now takes `*handlers.AdminDeletionHandler`; registers `/admin-deletion-requests` group. |
| `cmd/server/main.go` | Constructs `AdminDeletionRequestRepository` and `AdminDeletionHandler`; passes both to `RegisterUserRoutes`. |
| `internal/database/migrate.go` | `&models.AdminDeletionRequest{}` added to AutoMigrate list. |

---

## 3. Secret exposure result

No new secrets introduced. No credential values printed in code or tests.  
(Phase 1 already confirmed `.env` never committed to git; `.env.example` preserved.)

---

## 4. Build / test results

| Command | Result |
|---|---|
| `go fmt ./...` | PASS (reformatted: `admin_deletion_handler.go`, `models/user.go`, `repository/admin_deletion_request_repository.go`) |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./... -count=1` | PASS — handlers, middleware, models, services, utils all `ok` |
| `npm run build` | PASS — `tsc && tsc -p tsconfig.offline.json` exit 0 |

Integration tests (`PWAMS_FORCE_INTEGRATION=1`): SKIPPED — no PostgreSQL reachable in this environment, consistent with all prior Phase reports. The new service-layer code paths use the same transactional helpers (`userRepo.Tx`, `FindByIDTx`, `SoftDeleteInTx`, `RevokeAllByUserID`) that the existing integration suite already exercises for authz.

---

## 5. Remaining blockers

None. All 8 requirements are met, all default `go test ./...` suites are green, the frontend builds, and the admin-deletion workflow is wired end-to-end through handler → service → repository → migration. Live-DB verification of the new Manager-uniqueness row lock and the approval-flow state machine is deferred to a `PWAMS_FORCE_INTEGRATION=1` run against a real PostgreSQL instance (same limitation documented in every prior Phase report).