# PHASE 4O — DATABASE BACKUP PROCEDURE (NOT EXECUTED)

Status: **PROCEDURE ONLY.** No backup of any production database has been
claimed or created by this phase. Per the Phase 4 rules: *no backup, no
production migration.* This document defines the gate that must be
satisfied — with real artifacts — before `cmd/migrate up` is ever run
against the live database.

## Target database

`pwams_db` (canonical data in schema `pwams_user`; legacy `public` schema
is included in the dump automatically because `pg_dump` is database-wide).

## Command (PowerShell, PostgreSQL 18 local install)

```powershell
$stamp   = Get-Date -Format 'yyyyMMdd_HHmmss'
$dir     = 'E:\PWAMS\backups'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$file    = Join-Path $dir "pwams_db_$stamp.dump"

$env:PGPASSWORD = '<operator password from .env>'
& 'C:\Program Files\PostgreSQL\18\bin\pg_dump.exe' `
    -h localhost -U pwams_user -d pwams_db `
    -Fc -Z 9 -f $file

# Integrity: checksum + catalog listing
$hash = (Get-FileHash $file -Algorithm SHA256).Hash
& 'C:\Program Files\PostgreSQL\18\bin\pg_restore.exe' --list $file `
    | Select-Object -First 20
```

Artifacts to record (this section, filled in at execution time):

| Field | Value |
|---|---|
| Database name | pwams_db |
| Backup command | pg_dump -Fc -Z 9 (above) |
| Backup file | `pwams_db_<timestamp>.dump` |
| SHA-256 | _record at execution_ |
| Timestamp | _record at execution_ |
| Row-count snapshot | the Phase 4N section-4 table, captured at the same timestamp |

## Restore verification (mandatory — no claims without it)

Restore into a scratch database (never over anything existing):

```powershell
$env:PGPASSWORD = '<operator password>'
& 'C:\Program Files\PostgreSQL\18\bin\createdb.exe' `
    -h localhost -U pwams_user pwams_db_restore_check
& 'C:\Program Files\PostgreSQL\18\bin\pg_restore.exe' `
    -h localhost -U pwams_user -d pwams_db_restore_check `
    --no-owner --role=pwams_user $file
```

Then compare against the live snapshot:

```sql
SELECT (SELECT count(*) FROM users)  AS users,      -- expect 581
       (SELECT count(*) FROM persons) AS persons,   -- expect 237
       (SELECT count(*) FROM audit_logs) AS audit;  -- expect 1476
SELECT rolconfig FROM pg_roles WHERE rolname = 'pwams_user'; -- search_path intact
```

Pass criteria: restore completes without errors, the counts match the
snapshot, and `pg_restore --list` shows the full table set
(roles, users, persons, ..., schema_migrations when present).
Afterwards drop the scratch database:

```powershell
& 'C:\Program Files\PostgreSQL\18\bin\dropdb.exe' `
    -h localhost -U pwams_user pwams_db_restore_check
```

## Gate

The production migration may only proceed when ALL of the following are
true and evidenced in this file (or its execution log):

1. Backup file exists with a recorded SHA-256 and timestamp.
2. Restore verification passed into `pwams_db_restore_check`.
3. Row-count snapshot matches the live database at backup time.
4. `cmd/migrate verify` on `pwams_db` reports the expected pending set
   (000006-000009) and no drift.

Anything else ⇒ STOP, do not migrate.
