# PWAMS Test Execution Report

- **Date**: 2026-09-29
- **Tester / executor**: Cline (automated agent, HTTP-level execution)
- **Environment**: Local dev instance http://127.0.0.1:8081 built from this repo (commit 4c8a8ba); PostgreSQL localhost:5432/pwams_db; SMTP sandbox Mailpit 127.0.0.1:1025 (SMTP AUTH enabled); QA accounts from cmd/qaseed. NOT a confirmed staging/live environment.
- **Harness**: `scripts/qa_run_tests.py` + `scripts/qa_exec_a..d.py`
- **Cases in matrix**: 232
- **Executed**: 231 (**Pass 231**, **Fail 0**)
- **Blocked / not executed**: 1

> Automated HTTP-level execution only. No browser/visual case was executed and no human tester has signed this off.

## Results per module

| Module sheet | Cases | Pass | Fail | Blocked | Pass rate |
|---|---:|---:|---:|---:|---:|
| Account-Activation | 6 | 6 | 0 | 0 | 100% |
| Aid-Requests | 19 | 19 | 0 | 0 | 100% |
| Audit-Logs | 6 | 6 | 0 | 0 | 100% |
| Auth-Login | 16 | 16 | 0 | 0 | 100% |
| Care-Provided | 16 | 16 | 0 | 0 | 100% |
| Dashboard | 5 | 4 | 0 | 1 | 100% |
| Donations | 18 | 18 | 0 | 0 | 100% |
| Donors | 17 | 17 | 0 | 0 | 100% |
| File-Upload | 8 | 8 | 0 | 0 | 100% |
| Loan-Repayments | 8 | 8 | 0 | 0 | 100% |
| Loans | 17 | 17 | 0 | 0 | 100% |
| Localization-i18n | 4 | 4 | 0 | 0 | 100% |
| Messages | 10 | 10 | 0 | 0 | 100% |
| Notifications | 6 | 6 | 0 | 0 | 100% |
| Offline-Sync | 6 | 6 | 0 | 0 | 100% |
| Persons-Beneficiaries | 17 | 17 | 0 | 0 | 100% |
| Reports | 9 | 9 | 0 | 0 | 100% |
| Revenue | 7 | 7 | 0 | 0 | 100% |
| Security-RBAC | 9 | 9 | 0 | 0 | 100% |
| Students | 18 | 18 | 0 | 0 | 100% |
| User-Management | 10 | 10 | 0 | 0 | 100% |
| **TOTAL** | **232** | **231** | **0** | **1** | **100%** |

## Failures (0)

No automated case failed in this run.

## Blocked / not executed (1)

- **No automated executor mapped (0)**: none - browser/visual or human-judgement cases.
- **Environment prerequisite missing (1)**: DASH-005
  - DASH-005: Viewport/breakpoint rendering cannot be asserted over HTTP; requires a real browser at mobile/tablet widths.

## How to reproduce

```powershell
# 1. Mailpit SMTP sandbox with AUTH (the app uses smtp.PlainAuth):
Start-Process .\bin\mailpit.exe -ArgumentList '--smtp-auth-file','qa_mailpit_auth.txt','--smtp-auth-allow-insecure'
# 2. Full 232-case run + workbook/markdown export:
python scripts/qa_run_tests.py
#    one chunk only (Auth-Login / Activation / Users / Dashboard):
python scripts/qa_run_tests.py b
#    re-render the deliverables from qa_results.json without tests:
python scripts/qa_run_tests.py export
```

QA accounts come from `cmd/qaseed` (password `QaPassw0rd!2026`); the harness clears failed-login counters/lockouts in Postgres before the run and after every server recycle, so the AUTH-005 lockout cannot cascade into later cases.

## Honesty statement

- Every Pass/Fail above was captured live from the running server; nothing is extrapolated.
- `Blocked` rows were not executed and are not Pass.
- `Tested By` reads `Cline (automated agent, HTTP-level)`, not a human name - human sign-off is still required.
- The target is a local dev instance, not a confirmed staging/live environment.
