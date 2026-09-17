-- Rollback: PWAMS Schema Migration v8 (Data Integrity Hardening)
-- Removes the foreign key constraints, CHECK constraints, and soft-delete
-- columns added in v8.

-- Query support indexes
DROP INDEX IF EXISTS idx_audit_logs_created_at;

-- Soft delete columns
DROP INDEX IF EXISTS idx_loan_repayments_deleted_at;
ALTER TABLE loan_repayments DROP COLUMN IF EXISTS deleted_at;
DROP INDEX IF EXISTS idx_loans_deleted_at;
ALTER TABLE loans DROP COLUMN IF EXISTS deleted_at;
DROP INDEX IF EXISTS idx_care_provided_deleted_at;
ALTER TABLE care_provided DROP COLUMN IF EXISTS deleted_at;

-- Financial CHECK constraints
ALTER TABLE aid_requests DROP CONSTRAINT IF EXISTS chk_aid_requests_approved_amount_non_negative;
ALTER TABLE aid_requests DROP CONSTRAINT IF EXISTS chk_aid_requests_requested_amount_non_negative;
ALTER TABLE revenue_records DROP CONSTRAINT IF EXISTS chk_revenue_records_amount_non_negative;
ALTER TABLE donations DROP CONSTRAINT IF EXISTS chk_donations_amount_non_negative;
ALTER TABLE loan_repayments DROP CONSTRAINT IF EXISTS chk_loan_repayments_paid_non_negative;
ALTER TABLE loan_repayments DROP CONSTRAINT IF EXISTS chk_loan_repayments_amount_positive;
ALTER TABLE loans DROP CONSTRAINT IF EXISTS chk_loans_interest_rate_non_negative;
ALTER TABLE loans DROP CONSTRAINT IF EXISTS chk_loans_amount_positive;
ALTER TABLE care_provided DROP CONSTRAINT IF EXISTS chk_care_provided_amount_non_negative;

-- Foreign key constraints
ALTER TABLE loan_repayments DROP CONSTRAINT IF EXISTS fk_loan_repayments_loan;
ALTER TABLE loans DROP CONSTRAINT IF EXISTS fk_loans_created_by;
ALTER TABLE loans DROP CONSTRAINT IF EXISTS fk_loans_person;
ALTER TABLE care_provided DROP CONSTRAINT IF EXISTS fk_care_provided_created_by;
ALTER TABLE care_provided DROP CONSTRAINT IF EXISTS fk_care_provided_person;
ALTER TABLE care_provided DROP CONSTRAINT IF EXISTS fk_care_provided_aid_request;
