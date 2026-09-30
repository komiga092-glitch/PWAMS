# PHASE 4N — DATA MIGRATION PLAN (NOT EXECUTED)

Status: **PLAN ONLY — no statement in this document has been run against
`pwams_db`.** Production migration remains unapproved. Evidence base:
PHASE 4A read-only audit (completed) + dedicated `pwams_test` validation
(PHASE 4F, completed).

## 1. Canonical schema

`pwams_user` — resolved by the connection `search_path` (`"$user", public`),
verified as `current_schema()`, and the only schema whose structure matches
the GORM models (19 tables + `admin_deletion_requests` added by Phase 2).

## 2. Legacy schema

`public` — historical data ending 2026-09-14 (persons 629 rows all created
2026-09-12). **Not merged, not copied, not modified, not dropped.** It stays
as a frozen snapshot. Any later decision to retire it is a separate,
explicitly approved project.

## 3. Tables affected by THIS plan (canonical schema only)

| Object | Action |
|---|---|
| `organizations` | created by 000006 (validated) |
| `users.organization_id` + FK + index | created by 000006 (validated) |
| 10 tenant tables: `persons, students, donors, donations, aid_requests, care_provided, loans, loan_repayments, revenue_records, audit_logs` | gain `tenant_id → organizations(id)` FK from 000006 (validated) |
| `roles` (`Partner` orphan row) | reconciled by 000007 (validated) |
| `users` (Manager uniqueness trigger) | 000008 (validated) |
| `loan_repayments`, `roles` columns | 000009 model alignment (validated) |

## 4. Row counts (authoritative, 2026-09-20 audit)

| Table | pwams_user (canonical) | public (legacy, frozen) |
|---|---|---|
| users | 581 | 33 |
| roles | 9 | 8 |
| persons | 237 | 629 |
| students | 116 | 0 |
| donors | 124 | 0 |
| donations | 115 | 0 |
| aid_requests | 3 | 0 |
| care_provided | 2 | 0 |
| loans | 2 | 0 |
| loan_repayments | 0 | 0 |
| revenue_records | 3 | 0 |
| audit_logs | 1476 | 145 |
| sessions | 156 | 54 |

## 5. Duplicate handling

- ID collisions between schemas: irrelevant — the legacy schema is never
  merged, so cross-schema collisions do not arise.
- Within the canonical schema: zero non-NULL `tenant_id` rows, zero
  organizations — no tenant duplication exists. `uk_loan_repayment_installment`
  (000009) is fail-closed: if duplicates ever appear, `cmd/migrate
  repair-loan-repayments` reports them read-only first, deletes only with
  `--confirm`, and keeps the earliest row per the documented 000004 rule.
- Partner role: verified 0 live Partner users (only soft-deleted test
  accounts). 000007 therefore takes the merge path (re-point soft-deleted
  rows, drop orphan role) — no live role change; fail-closed if the
  situation ever changes before migration day.

## 6. Foreign key handling

All new FKs are additive and validated on `pwams_test`:
`fk_users_organization` (RESTRICT) and `fk_<table>_tenant_organization`
(RESTRICT, 10 tables). Because every existing `tenant_id`/`organization_id`
is NULL, no backfill is needed before the FKs attach; NULL stays legal
(platform scope / not-yet-assigned).

## 7. ID collision handling

User/role/person IDs are UUIDv4 generated per row; no auto-increment
sequence exists, so no collision class exists within the canonical schema.
`schema_migrations.version` is the only sequence-like key and is
tool-managed.

## 8. Organization mapping (bootstrap — NOT automatic)

No organization is created and no user is assigned by any migration
(enforced and tested: `TestExistingDataPreservedThroughTenancyMigrations`
asserts `organizations` stays empty). The documented bootstrap rule is:

1. Operator/admin creates the first organization(s) via an explicit,
   audited action (name + code).
2. Existing users are assigned only under an auditable mapping rule.
   Creator provenance for existing rows is `komikukan` plus Phase-2/3
   `authz_*` test accounts — the data does NOT encode real-world
   organization membership. Therefore:
   - no user may be auto-assigned by a script using creation-date or
     creator-id heuristics;
   - assignment is a per-user admin action (or an explicitly reviewed
     mapping table signed off by the organization owner), recorded in
     `audit_logs` with actor, action, entity, entity_id.
3. Until assignment, `organization_id IS NULL` = platform scope — exactly
   the pre-tenancy behavior; no user loses access during rollout.

## 9. User mapping

| Role (live counts) | Mapping rule |
|---|---|
| Super Admin (1) | stays NULL — platform level, never forced into an NGO (PHASE 4G rule) |
| Manager (1, `audit_mgr`) | assigned to its NGO's organization only when that organization is bootstrapped; the 000008 trigger then enforces one active Manager inside that scope |
| Admin (3), Staff (564) | per-user assignment under the auditable rule in §8; NULL until then |
| Volunteer/Donor/Beneficiary/Student (12 total) | same rule |

## 10. TenantID mapping

Decision (PHASE 4H): retain `tenant_id` as the project's tenant
abstraction; `organizations.id` is its value domain. No rename, no new
column, no data rewrite — the FK added by 000006 already constrains
`tenant_id` to organizations. Application enforcement then scopes queries
by `tenant_id = <user's organization_id>` (work items 4H/4I/4L, not yet
implemented).


## 11. Rollback strategy

- Every migration ships a `.down.sql`; 000006/000007/000009 downs are
  additive-reversal only and never delete application data (verified on
  `pwams_test`).
- `schema_migrations` checksums make any unplanned change fail closed.
- Ultimate rollback = restore the pre-migration backup (Phase 4O
  procedure). Because 000006-000009 change little data, a logical
  `pg_restore` of the single backup fully reverses the upgrade.

## 12. Backup strategy

Phase 4O procedure (`docs/phase4_backup_procedure.md`): custom-format
`pg_dump -Fc` of `pwams_db`, checksummed, restore-verified into a scratch
database before `cmd/migrate up` is allowed against production.
No backup, no migration.

## 13. Validation strategy

1. `pwams_test` full migration suite (already green, 12/12 tests).
2. Pre-migration production snapshot: row counts (section 4) captured with
   the backup timestamp.
3. Post-migration verification, all must hold:
   - same row counts for every application table (except at most 2 new
     `migration` audit markers);
   - `SELECT count(*) FROM organizations` = number bootstrapped by the
     operator (0 if none);
   - every `tenant_id`/`organization_id` still NULL (no fabricated
     assignment);
   - `cmd/migrate verify` clean; server startup with `APP_ENV=production`
     succeeds;
   - `cmd/migrate status` shows 000001-000009 applied, checksums ok.
4. Staged execution: `up` to 5 (baseline parity), verify the server boots,
   then `up` to 9 — the runner supports staging (`MigrateUpTo`).

## STOP conditions (any one halts the plan - do not improvise)

- Live Partner users exist at migration time (000007 raises; resolve
  explicitly first).
- Duplicate `(loan_id, installment_number)` rows (000009 raises; run the
  documented repair after a fresh backup).
- Post-migration counts differ from the snapshot beyond the audit
  markers: restore from backup and investigate.
- Any `pwams_test` regression before migration day: fix and re-validate;
  never "fix production to match tests".

