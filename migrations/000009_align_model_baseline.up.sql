-- PHASE 4F — align the migration baseline with the application model
-- (migration 000009).
--
-- 000001 predates later model-driven schema changes: GORM AutoMigrate
-- evolved the canonical schema beyond the original SQL. A deployment that
-- relies exclusively on tracked migrations must still get a schema the
-- application model can use, so this migration idempotently adds what the
-- models require:
--
--   * loan_repayments: the installment schedule columns
--     (installment_number, due_date, paid_amount, paid_at,
--     payment_reference, notes) — the original one-shot repayment design
--     had none of them;
--   * loan_repayments.payment_date loses NOT NULL — the current
--     LoanRepayment model has no payment_date field (replaced by the
--     installment schedule), so GORM inserts never supply it and the
--     legacy constraint would reject every repayment created by the
--     application. The column and its data are KEPT (additive change;
--     the live canonical table holds 0 rows per the Phase 4A audit);
--   * roles.description + roles.deleted_at (+ index) — the Role model is
--     soft-deletable with a description; SeedDefaultRoles (GORM
--     FirstOrCreate) writes both columns on every server startup.
--
-- Fail-closed duplicate protection: the final CREATE UNIQUE INDEX
-- (uk_loan_repayment_installment) aborts if duplicate
-- (loan_id, installment_number) rows exist; the operator then runs
-- `go run ./cmd/migrate repair-loan-repayments` (documented repair,
-- migration 000004) after taking a backup. Startup never silently
-- deletes duplicates (see internal/database/repair.go).

ALTER TABLE loan_repayments
    ADD COLUMN IF NOT EXISTS installment_number INT,
    ADD COLUMN IF NOT EXISTS due_date TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS paid_amount NUMERIC(15,2) NOT NULL DEFAULT '0',
    ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS payment_reference VARCHAR(255),
    ADD COLUMN IF NOT EXISTS notes TEXT;

-- PHASE 4P baseline-safety: on a fresh 000001 baseline payment_date exists
-- with NOT NULL and must be relaxed (the model never writes it); on the
-- canonical AutoMigrate-shaped schema the column does not exist at all and
-- there is nothing to relax. Verified during the PHASE 4P staged rollout.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'loan_repayments'
          AND column_name = 'payment_date'
    ) THEN
        ALTER TABLE loan_repayments ALTER COLUMN payment_date DROP NOT NULL;
    ELSE
        RAISE NOTICE '000009: loan_repayments has no payment_date column on this baseline; relaxation skipped';
    END IF;
END $$;

ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles(deleted_at);

CREATE UNIQUE INDEX IF NOT EXISTS uk_loan_repayment_installment
    ON loan_repayments (loan_id, installment_number);
