-- DAY1 / PHASE 6 — one-off data repair, run deliberately by an operator.
--
-- The application startup no longer deletes data automatically (see
-- internal/database/migrate.go): when duplicate (loan_id, installment_number)
-- rows exist, startup fails and points here. This migration removes every
-- later duplicate deterministically (created_at, then id), keeping the first
-- row per group, so the composite unique index
-- uk_loan_repayment_installment can be created.
--
-- Review before running: it deletes rows. Take a backup first, e.g.
--   pg_dump -d "$DB_NAME" -t loan_repayments > loan_repayments_backup.sql

DELETE FROM loan_repayments a
USING loan_repayments b
WHERE a.loan_id = b.loan_id
  AND a.installment_number = b.installment_number
  AND (a.created_at > b.created_at
       OR (a.created_at = b.created_at AND a.id > b.id));