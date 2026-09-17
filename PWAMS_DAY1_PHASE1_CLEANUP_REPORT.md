# PWAMS — DAY 1 / PHASE 1 — CLEANUP REPORT

Date: 2026-09-12  
Scope: Repository cleaning only — no application rewrite, no business-logic change, no architecture change.  
Architecture preserved: Go 1.26.6 / Gin / GORM / PostgreSQL + HTMX / HTML / CSS / TypeScript / PWA.

## 1. Removed files / categories (confirmed non-source artifacts only)

| Category | Files / paths | Notes |
|---|---|---|
| VCS metadata | `.git/` | Removed last, per instructions. Includes local IDE-checkpoint refs (`refs/cline/checkpoints/*`). Last pushed state remains on GitHub: `origin/main` @ `424353b`. |
| Node modules | `node_modules/` | Build dependency, restorable via `npm ci` (package-lock.json preserved). Removed after successful build validation. |
| Compiled binaries | `pwams_server.exe` (root, ~52 MB), `bin/pwams-server-test.exe` (~52 MB) | Build artifacts; gitignored by `*.exe` / `bin/`. Not referenced by any build/source file. |
| `bin/` build artifacts | `bin/` (exe + `server-test-out.log`, `server-test-err.log`) | Entire directory removed. |
| `tmp/` | `tmp/` | Empty temp directory. |
| Temporary investigation logs (*.log) | `baseline_probe.log`, `build_1266.log`, `build_baseline.log`, `extract_go.log`, `go1266_probe.log`, `govuln_1266.log`, `govuln_baseline.log`, `mod_tidy.log`, `test_baseline.log` | One-off probe/vulncheck logs (gitignored by `*.log`). |
| Temporary *.exit files | `baseline_probe.log.exit`, `build_1266.log.exit`, `build_baseline.log.exit`, `go1266_probe.log.exit`, `govuln_1266.log.exit`, `govuln_baseline.log.exit`, `mod_tidy.log.exit`, `test_baseline.log.exit` | Exit-code companions of the above. |
| Temporary git/debug text files | `git_status.txt`, `git_diff.txt`, `db_result.txt`, `sync_matrix_results.txt`, `my_toolchain.diff` | Investigation captures/diffs. |
| Temporary PowerShell investigation scripts | `run_capture.ps1`, `run_go1266.ps1`, `expand_go.ps1`, `PWAMSix_sync.ps1` | Reference-checked: no application/build dependency. `expand_go.ps1`/`run_go1266.ps1` referenced machine-specific toolchain paths; `PWAMSix_sync.ps1` was a one-off code-mutation script (already applied). |
| Unused captured/debug page | `login_page.html` | Root-level page capture; reference check: unused (real page = `web/templates/login.html` + `components/login_templ.go`). |
| Unused Java artifact | `sa.jar` | Reference check across repo: no source reference. |
| Environment file (SECURITY) | `.env` | NEVER included in delivery, per instructions. Excluded before delivery; recreate from `.env.example` (dev must re-enter local values for `APP_ENV`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `SUPER_ADMIN_*`). `.env` was never committed to git (verified: 0 commits, 0 blobs, incl. checkpoints). |

## 2. Preserved files / categories (verified present after cleanup)

- Application source: `cmd/`, `internal/` (config, constants, database, handlers, middleware, models, repository, routes, services, utils, email — incl. earlier-phase files `internal/services/ownership.go` and the `authz_*_test.go` test suite).
- Web / PWA: `web/` (templates, static CSS/JS/TS, `web/src/offline` TypeScript incl. `i18n.ts`, compiled `web/static/js/offline/*` outputs, ESM marker `web/static/js/offline/package.json`, `manifest.webmanifest`, service worker, offline.html).
- Database: `migrations/` (`000001_init_schema` up/down, `000002_drop_audit_ip` up/down, `.gitkeep`).
- Documentation: `docs/` (`docs.go`, `swagger.json`, `swagger.yaml`).
- Scripts: `scripts/backup.sh` (protected), `scripts/offline_session_test.mjs` (REAL application test: offline session lifecycle STEP 12.5/12.6 — imported by `node --test`, documented in STEP-12/15 reports — preserved, not a one-off).
- Storage: `storage/uploads/.gitkeep` (gitignored upload payload excluded, .gitkeep kept).
- Build/CI: `go.mod`, `go.sum`, `package.json`, `package-lock.json`, `tsconfig.json`, `tsconfig.offline.json`, `Dockerfile`, `docker-compose.yml`, `nginx.conf`, `.github/workflows/ci-cd.yml`, `.gitignore`, `.dockerignore`.
- Environment: `.env.example` (kept; audited — placeholder-style values only, no long random literals).
- Root documentation preserved (not in removal taxonomy; candidates to relocate under `docs/` in a later housekeeping pass): 15 × `PWAMS_*_REPORT.md` (CRA, FINAL_TESTING, SBOM_SECURITY, STEP_12/15 series), `SBOM-CycloneDX.json`, `SBOM-SPDX.json`, `SBOM_LICENSE_REPORT.md`.

## 3. Secret exposure result

Method: filename scan of all history (`.env` / `.pem` / `id_rsa` / `.jks` / `credential` / `.key`), pickaxe content scan of all 288 commits (`git log -G`, commit IDs only, no content printed), tracked-tree grep, working-tree grep (paths only), `.env` vs `.env.example` comparison (hashes + key names only — no values printed at any point).

| Check | Result |
|---|---|
| `.env` committed to git? | **No** — 0 commits, 0 blobs (including IDE checkpoint commits). Existed on local disk only. |
| `debug_flow.ps1` (deleted temp script, snapshotted in local IDE-checkpoint commits `7cf822d`…`66562ec` under `refs/cline/checkpoints/*`) | Contained a secret-shaped line; **never reachable from any remote branch** (`git branch -r --contains` empty) — never pushed. Fully purged by `.git/` removal. |
| `origin/main` history (GitHub) | **Clean** — no commit ever added/removed a secret-shaped line; no `.env` / key / certificate files ever. |
| `scripts/backup.sh` | Clean — `DB_PASSWORD` / `export PGPASSWORD` use env/variable references only; history clean. |
| `internal/constants/messages.go` (line 20) | `ErrInvalidPassword` — error-message constant, **not** a secret. |
| `.env.example` | Placeholder-style values only (no long random literals) — safe, kept. |

**Verdict: credential rotation NOT required.** No secret ever reached the repository or GitHub (`origin`). The `.env` values were local-disk-only dev configuration; the only history copy of a secret-shaped value (`debug_flow.ps1`) lived in local-only checkpoint refs destroyed with `.git/`. (Optional belt-and-braces: rotating the local dev DB/admin passwords is harmless but not mandated.)

## 4. Build / test results (run after artifact cleanup, before `node_modules/` removal — `tsc` requires it)

| Command | Result | Detail |
|---|---|---|
| `go fmt ./...` | **PASS** (exit 0) | No files required reformatting. |
| `go test ./...` | **PASS** (exit 0) | `handlers`, `middleware`, `models`, `services`, `utils` = ok; DB-required integration tests auto-skip by design (`internal/services/test_db_helper_test.go`: `t.Skipf` when no DB reachable). |
| `go vet ./...` | **PASS** (exit 0) | No findings. |
| `go build ./...` | **PASS** (exit 0) | No stray artifacts left in the tree. |
| `npm run build` | **PASS** (exit 0) | `tsc && tsc -p tsconfig.offline.json` (root + offline configs) — regenerated tracked JS outputs under `web/static/js/`. |

Final junk verification: recursive search for `*.exe` / `*.log` / `*.jar` / `*.zip` / `*.exit` / `*.diff` / `*.tmp` / `*.bak` → **empty**. `.env` absent; `.env.example` present; all protected paths verified present.

## 5. Remaining blockers

None blocking Phase-1 delivery. Non-blocking notes for the operator:

1. **No version control**: `.git/` was removed per instructions. The working tree is unversioned; the last pushed state remains on GitHub at `origin/main` @ `424353b` (local tree included uncommitted improvements from earlier phases, which exist only as current files). Recommended next administrative step (not Phase 2): `git init` + initial commit of the clean tree.
2. **Restore Node toolchain before `npm run build` / `npm run dev`**: `npm ci` (package-lock.json preserved; only `typescript` is a devDependency — the offline test needs only `node --test`).
3. **Recreate `.env`** from `.env.example` before running the server locally (keys needing local dev values: `APP_ENV`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `SUPER_ADMIN_USERNAME`, `SUPER_ADMIN_EMAIL`, `SUPER_ADMIN_PASSWORD`).
4. **Housekeeping candidates (preserved deliberately)**: 15 root-level `PWAMS_*_REPORT.md` + `SBOM-*.json` / `SBOM_LICENSE_REPORT.md` are documentation, not in the confirmed removal taxonomy — consider moving under `docs/` in a later pass.
5. Root report `PWAMS_DAY1_PHASE1_CLEANUP_REPORT.md` is the only new file created by this phase.

— End of Day 1 / Phase 1 report. Phase 2 not started. —
