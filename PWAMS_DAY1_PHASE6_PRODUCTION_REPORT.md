# PWAMS — DAY 1 / PHASE 6 — PRODUCTION HARDENING REPORT

Date: 2026-09-13
Scope: verified production blockers only — Docker, migration strategy, CI, secrets.

---

## 1. Docker

### Findings (verified against the source, not assumptions)

| # | Blocker | Evidence | Fix |
|---|---|---|---|
| 1.1 | Unpinned images allowed drifting production builds | `Dockerfile`: `golang:1.26-alpine`, `alpine:3.19`; `docker-compose.yml`: `nginx:alpine`, `postgres:16-alpine` | Pinned: `golang:1.26.6-alpine` (exactly matches `go.mod` `go 1.26.6`), `alpine:3.19.1`, `nginx:1.27.4-alpine`, `postgres:16.6-alpine`. CI `postgres` service pinned to `postgres:16.6-alpine` as well. |
| 1.2 | App port published on all host interfaces | `ports: "8080:8080"` let clients bypass TLS-terminating nginx | Changed to `127.0.0.1:8080:8080` (loopback only); public traffic must traverse nginx 80/443. |
| 1.3 | nginx referenced TLS files that were never mounted | `nginx.conf` requires `/etc/nginx/ssl/tls.crt` + `tls.key`; compose mounted **only** `nginx.conf` — nginx would fail to start (or serve no TLS) | Added read-only mount `./nginx/ssl:/etc/nginx/ssl:ro` with documented host paths. **No certificates were invented or committed** — the operator supplies real ones. |
| 1.4 | nginx `/static/` alias pointed at a path that does not exist in the nginx container | `location /static/ { alias /app/web/static/; }` — only the app image contains `/app/web` (Dockerfile `COPY --from=builder /app/web`); the app serves static via `router.Static("/static", "web/static")` | Replaced alias with `proxy_pass http://pwams` (+ cache headers), consistent with every other location. |
| 1.5 | PostgreSQL exposure | compose has **no** `ports:` mapping for postgres (internal network only) — verified correct, comment retained | No change needed. |
| 1.6 | nginx ↔ compose consistency | upstream `server app:8080` matches service name `app`; health endpoint `/health` exists (`routes.go`), so the Dockerfile `HEALTHCHECK` target is valid | Verified, no change. |

### Required production certificate paths (operator-provided)

```
host: ./nginx/ssl/tls.crt  -> container: /etc/nginx/ssl/tls.crt  (full chain, PEM)
host: ./nginx/ssl/tls.key  -> container: /etc/nginx/ssl/tls.key  (private key, chmod 0600)
```

nginx fails loudly when either file is missing (documented in `nginx.conf`).
Obtain certificates from a real CA (e.g. Let's Encrypt); nothing is generated
in this repository.

## 2. Migration

### Findings

| # | Blocker | Evidence | Fix |
|---|---|---|---|
| 2.1 | **Dual migration strategy** | `internal/database/migrate.go` runs GORM `AutoMigrate` at startup (called from `cmd/server/main.go:52`) while `migrations/*.sql` exists as a second schema source — and **nothing in the codebase executes the SQL files** (no runner, no version tracking; the Dockerfile merely copies them). The two sources had already drifted. | Documented and converged: AutoMigrate is the runtime authority; the SQL set is the external `migrate`-tool/provisioning path and is now kept in sync with the model (see 2.3–2.5). |
| 2.2 | **Destructive data deletion during startup** | `Migrate()` ran `DELETE FROM loan_repayments …` (duplicate cleanup) on **every** boot | Replaced with a non-destructive duplicate **check** that fails startup with operator instructions; the actual dedup is now a reviewed, operator-run SQL migration (`migrations/000004_loan_repayment_dedup.up.sql`, with backup instructions). Zero `DELETE` statements remain in startup code. |
| 2.3 | Fresh provisioning created the forbidden audit IP column | `000001_init_schema.up.sql` line 269: `ip_address VARCHAR(45)` in `audit_logs` | Removed from the provisioning schema (with an explanatory comment); the model and runtime already exclude it. |
| 2.4 | **Rollback silently reintroduced the forbidden audit IP field** | `000002_drop_audit_ip.down.sql` ran `ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45)` | Down migration is now a documented no-op — rollback can never re-add the PII column. (Runtime `DropColumn` guard in `Migrate()` kept as defense in depth.) |
| 2.5 | `audit_logs` must not contain IP address | Model has no IP field; runtime drops legacy column; provisioning schema no longer creates it; rollback no longer restores it — verified by grep across `migrations/*.sql` | Verified end-to-end. |
| 2.6 | No destruction of existing production data | `000004.up` deletes only duplicate `loan_repayments` rows **when an operator chooses to run it** (with a backup step in its header); the `users.status='Pending' → 'Active'` normalization at startup is a non-destructive UPDATE retained from Phase 2; `DropColumn ip_address` is the mandated PII remediation | No other startup data mutation/deletion exists. |

Note: `000003_admin_deletion_requests.up.sql` already matched the GORM model
(its header documents the intent) — no change needed.

## 3. CI (`.github/workflows/ci-cd.yml`)

| # | Blocker | Evidence | Fix |
|---|---|---|---|
| 3.1 | Go version misaligned | CI used `GO_VERSION: "1.26"` while `go.mod` requires `go 1.26.6` | `GO_VERSION: "1.26.6"` — now identical to go.mod and the Dockerfile builder image. |
| 3.2 | Go vet/build not run in CI | only `golangci-lint` + `go test` | Added `go vet ./...` and `go build ./...` steps to the Test job. |
| 3.3 | npm build not run in CI | no frontend job existed | New `frontend` job: `setup-node` (22) → `npm ci` → `npm run build`; the Docker `build` job now `needs: [lint, test, frontend]`. |
| 3.4 | templ generation | The project **does** depend on generated templ files — `internal/routes/auth_routes.go` imports `web/templates/components` and renders `components.LoginPage(...)`; generated files carry `// templ: version: v0.3.1020` = `github.com/a-h/templ v0.3.1020` in go.mod | Added a CI step that installs the **pinned** CLI (`templ@v0.3.1020`), runs `templ generate`, and fails when committed generated files drift (`git diff --exit-code -- web/templates/components`). Validated locally: generation produced `updates=0` and SHA-256 hashes of all `*_templ.go` files were **identical before/after** → in sync. |
| 3.5 | No Python CI scripts | — | None created; all steps are plain shell. |

## 4. Secrets

| Check | Result |
|---|---|
| `.env` present in repository/delivery? | **No** — file does not exist on disk (verified `Test-Path .env` → False). |
| `.gitignore` coverage | `.env`, `.env.local`, `.env.production` ignored. |
| `.dockerignore` coverage | `.env` and `.env.*` excluded, `!.env.example` allow-listed — secrets cannot enter images. |
| `.env.example` contents | Placeholders only (`change_me_in_production`); contains no real credentials. |
| Secrets printed during this phase? | **No** — no secret values were printed; the report contains no credential values. CI uses throwaway ephemeral test-service credentials (documented as CI-only, not production secrets). |
| Production secret provisioning | Operator sets `DB_PASSWORD`, `SUPER_ADMIN_*`, SMTP values via host environment or a git-ignored `.env` consumed by `docker compose` interpolation — never committed. |

## 5. Validation results (all executed this session)

| Command | Result |
|---|---|
| `go fmt ./...` | PASS (exit 0) |
| `go vet ./...` | PASS (exit 0) |
| `go build ./...` | PASS (exit 0) |
| `go test ./...` | PASS (exit 0) — full suite against a **pristine** PostgreSQL test DB, including the new fail-fast duplicate check in `Migrate()` |
| `npm run build` | PASS (exit 0) |
| `templ generate` + hash comparison | PASS — committed generated files in sync with `templ@v0.3.1020` |
| migrations `ip_address` grep | PASS — column is only ever **dropped** (000002.up) or documented absent (000001 comment); no DDL creates it, rollback never re-adds it |
| `DELETE` in startup migration code | PASS — zero occurrences |

Not runnable on this host (documented honestly): `docker build` / `docker
compose config` / live `nginx -t` — Docker is not installed in the working
environment. All changed files were reviewed manually against the pinned
upstream image tags and nginx syntax; the compose/nginx changes are
declarative and were verified by inspection.

## 6. Files changed / added

**Changed:**
- `Dockerfile` — pinned `golang:1.26.6-alpine`, `alpine:3.19.1`
- `docker-compose.yml` — pinned `nginx:1.27.4-alpine`, `postgres:16.6-alpine`; app bound to `127.0.0.1:8080:8080`; added `./nginx/ssl:/etc/nginx/ssl:ro`
- `nginx.conf` — `/static/` proxied to the app; TLS mount paths documented
- `internal/database/migrate.go` — destructive startup `DELETE` → fail-fast duplicate check
- `migrations/000001_init_schema.up.sql` — no `ip_address` in `audit_logs`
- `migrations/000002_drop_audit_ip.down.sql` — rollback no-op (PII never restored)
- `.github/workflows/ci-cd.yml` — Go 1.26.6; vet/build; templ drift check; frontend job

**Added:**
- `migrations/000004_loan_repayment_dedup.up.sql` — operator-run dedup (backup-first)
- `migrations/000004_loan_repayment_dedup.down.sql` — documented no-op

## 7. Remaining blockers / honest limitations

1. **TLS certificates must be supplied by the operator** at
   `./nginx/ssl/tls.{crt,key}` — intentionally not invented here.
2. Docker/`nginx -t` validation could not execute on this host (no Docker);
   compose and nginx changes were verified by source inspection only.
3. The deploy job remains a placeholder — production deployment targets are
   environment-specific and out of this phase's scope.
4. The dual migration strategy is now documented and the sources are aligned;
   full consolidation onto a single migration runner (e.g. golang-migrate)
   would be a larger architectural change and was deliberately **not**
   attempted in this phase.

Phase 7 not started.