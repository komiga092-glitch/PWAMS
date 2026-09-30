-- Rollback for 000009: removes the model-alignment additions. Only run on
-- dedicated test databases or after an explicit, backed-up operator
-- decision; dropping them breaks the application model and re-allows
-- duplicate loan repayment installments.
DROP INDEX IF EXISTS uk_loan_repayment_installment;

DROP INDEX IF EXISTS idx_roles_deleted_at;

-- Restore the legacy NOT NULL only when the column exists AND every
-- payment_date is populated; a database that never had the column (the
-- canonical AutoMigrate shape) or that already stored NULL payment_date
-- rows must not fail the rollback.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'loan_repayments'
          AND column_name = 'payment_date'
    ) THEN
        IF NOT EXISTS (SELECT 1 FROM loan_repayments WHERE payment_date IS NULL) THEN
            ALTER TABLE loan_repayments ALTER COLUMN payment_date SET NOT NULL;
        END IF;
    END IF;
END $$;

ALTER TABLE roles DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE roles DROP COLUMN IF EXISTS description;

ALTER TABLE loan_repayments
    DROP COLUMN IF EXISTS notes,
    DROP COLUMN IF EXISTS payment_reference,
    DROP COLUMN IF EXISTS paid_at,
    DROP COLUMN IF EXISTS paid_amount,
    DROP COLUMN IF EXISTS due_date,
    DROP COLUMN IF EXISTS installment_number;
