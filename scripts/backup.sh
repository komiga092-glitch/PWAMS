#!/usr/bin/env bash
set -euo pipefail

# PWAMS Database Backup Script
# Usage: ./backup.sh [daily|weekly|monthly]

BACKUP_TYPE="${1:-daily}"
BACKUP_DIR="/backups/pwams"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/pwams_${BACKUP_TYPE}_${TIMESTAMP}.sql.gz"

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-pwams}"
DB_NAME="${DB_NAME:-pwams}"
DB_PASSWORD="${DB_PASSWORD:-}"

RETENTION_DAILY=7
RETENTION_WEEKLY=4
RETENTION_MONTHLY=12

mkdir -p "${BACKUP_DIR}"

export PGPASSWORD="${DB_PASSWORD}"

echo "Starting ${BACKUP_TYPE} backup: ${BACKUP_FILE}"

pg_dump \
  -h "${DB_HOST}" \
  -p "${DB_PORT}" \
  -U "${DB_USER}" \
  -d "${DB_NAME}" \
  --no-owner \
  --no-privileges \
  --clean \
  --if-exists \
  | gzip > "${BACKUP_FILE}"

BACKUP_SIZE=$(stat -f%z "${BACKUP_FILE}" 2>/dev/null || stat -c%s "${BACKUP_FILE}" 2>/dev/null)
echo "Backup completed: ${BACKUP_FILE} (${BACKUP_SIZE} bytes)"

# Verify backup
if ! gzip -t "${BACKUP_FILE}"; then
  echo "ERROR: Backup verification failed" >&2
  exit 1
fi

# Prune old backups
case "${BACKUP_TYPE}" in
  daily)
    find "${BACKUP_DIR}" -name "pwams_daily_*.sql.gz" -mtime +${RETENTION_DAILY} -delete
    ;;
  weekly)
    find "${BACKUP_DIR}" -name "pwams_weekly_*.sql.gz" -mtime +$((RETENTION_WEEKLY * 7)) -delete
    ;;
  monthly)
    find "${BACKUP_DIR}" -name "pwams_monthly_*.sql.gz" -mtime +$((RETENTION_MONTHLY * 30)) -delete
    ;;
esac

echo "Pruning complete. Active backups:"
ls -la "${BACKUP_DIR}/" | grep "pwams_${BACKUP_TYPE}" || true

unset PGPASSWORD
