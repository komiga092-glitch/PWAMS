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
--
-- PHASE 4F note (idempotent + baseline-safe): on a baseline created by
-- 000001 the installment-schedule columns do not exist yet, so the
-- keep-earliest DELETE is skipped with a notice; 000009 adds the columns
-- and the unique index (and fails closed on real duplicates). On the
-- canonical schema the columns exist and the dedup runs as documented.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'loan_repayments'
          AND column_name = 'installment_number'
    ) THEN
        DELETE FROM loan_repayments a
        USING loan_repayments b
        WHERE a.loan_id = b.loan_id
          AND a.installment_number = b.installment_number
          AND (a.created_at > b.created_at
               OR (a.created_at = b.created_at AND a.id > b.id));
    ELSE
        RAISE NOTICE '000004: loan_repayments has no installment_number column on this baseline; dedup skipped (000009 adds the column and the unique index)';
    END IF;
END $$;
