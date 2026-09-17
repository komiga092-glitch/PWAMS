-- PWAMS Schema Migration v8: Data Integrity Hardening
-- Adds missing foreign key constraints, soft-delete columns, and CHECK
-- constraints to enforce referential integrity and prevent invalid financial
-- data.

-- ============================================================================
-- SOFT DELETE COLUMNS
-- CareProvided, Loan, and LoanRepayment were missing a deleted_at column,
-- meaning GORM performed hard deletes instead of soft deletes. This caused
-- records to be permanently removed, breaking sync propagation and audit
-- trails. Adding the column enables proper soft-delete semantics.
-- ============================================================================

ALTER TABLE care_provided ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_care_provided_deleted_at ON care_provided(deleted_at);

ALTER TABLE loans ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_loans_deleted_at ON loans(deleted_at);

ALTER TABLE loan_repayments ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_loan_repayments_deleted_at ON loan_repayments(deleted_at);

-- ============================================================================
-- FOREIGN KEY CONSTRAINTS
-- These tables were missing explicit REFERENCES clauses in the original
-- migration. GORM AutoMigrate may have created some, but we add them
-- explicitly here to guarantee database-level referential integrity.
-- ============================================================================

-- CareProvided → AidRequest (RESTRICT: cannot delete aid request with care records)
ALTER TABLE care_provided
    DROP CONSTRAINT IF EXISTS fk_care_provided_aid_request;
ALTER TABLE care_provided
    ADD CONSTRAINT fk_care_provided_aid_request
    FOREIGN KEY (aid_request_id) REFERENCES aid_requests(id)
    ON UPDATE CASCADE ON DELETE RESTRICT;

-- CareProvided → Person (RESTRICT: cannot delete person with care records)
ALTER TABLE care_provided
    DROP CONSTRAINT IF EXISTS fk_care_provided_person;
ALTER TABLE care_provided
    ADD CONSTRAINT fk_care_provided_person
    FOREIGN KEY (person_id) REFERENCES persons(id)
    ON UPDATE CASCADE ON DELETE RESTRICT;

-- CareProvided → CreatedBy User (RESTRICT: cannot delete user who created care records)
ALTER TABLE care_provided
    DROP CONSTRAINT IF EXISTS fk_care_provided_created_by;
ALTER TABLE care_provided
    ADD CONSTRAINT fk_care_provided_created_by
    FOREIGN KEY (created_by_id) REFERENCES users(id)
    ON UPDATE CASCADE ON DELETE RESTRICT;

-- Loan → Person (RESTRICT: cannot delete person with loans)
ALTER TABLE loans
    DROP CONSTRAINT IF EXISTS fk_loans_person;
ALTER TABLE loans
    ADD CONSTRAINT fk_loans_person
    FOREIGN KEY (person_id) REFERENCES persons(id)
    ON UPDATE CASCADE ON DELETE RESTRICT;

-- Loan → CreatedBy User (RESTRICT: cannot delete user who created loans)
ALTER TABLE loans
    DROP CONSTRAINT IF EXISTS fk_loans_created_by;
ALTER TABLE loans
    ADD CONSTRAINT fk_loans_created_by
    FOREIGN KEY (created_by_id) REFERENCES users(id)
    ON UPDATE CASCADE ON DELETE RESTRICT;

-- LoanRepayment → Loan (RESTRICT: cannot delete loan with repayments)
ALTER TABLE loan_repayments
    DROP CONSTRAINT IF EXISTS fk_loan_repayments_loan;
ALTER TABLE loan_repayments
    ADD CONSTRAINT fk_loan_repayments_loan
    FOREIGN KEY (loan_id) REFERENCES loans(id)
    ON UPDATE CASCADE ON DELETE RESTRICT;

-- ============================================================================
-- FINANCIAL CHECK CONSTRAINTS
-- Prevent negative financial amounts that would indicate data corruption.
-- These constraints work alongside application-level validation.
-- ============================================================================

-- CareProvided amount must be non-negative
ALTER TABLE care_provided
    DROP CONSTRAINT IF EXISTS chk_care_provided_amount_non_negative;
ALTER TABLE care_provided
    ADD CONSTRAINT chk_care_provided_amount_non_negative
    CHECK (amount >= 0);

-- Loan amount must be positive (a zero-value loan is meaningless)
ALTER TABLE loans
    DROP CONSTRAINT IF EXISTS chk_loans_amount_positive;
ALTER TABLE loans
    ADD CONSTRAINT chk_loans_amount_positive
    CHECK (loan_amount > 0);

-- Loan interest rate must be non-negative
ALTER TABLE loans
    DROP CONSTRAINT IF EXISTS chk_loans_interest_rate_non_negative;
ALTER TABLE loans
    ADD CONSTRAINT chk_loans_interest_rate_non_negative
    CHECK (interest_rate >= 0);

-- Loan repayment amount must be positive
ALTER TABLE loan_repayments
    DROP CONSTRAINT IF EXISTS chk_loan_repayments_amount_positive;
ALTER TABLE loan_repayments
    ADD CONSTRAINT chk_loan_repayments_amount_positive
    CHECK (amount > 0);

-- Loan repayment paid amount must be non-negative
ALTER TABLE loan_repayments
    DROP CONSTRAINT IF EXISTS chk_loan_repayments_paid_non_negative;
ALTER TABLE loan_repayments
    ADD CONSTRAINT chk_loan_repayments_paid_non_negative
    CHECK (paid_amount >= 0);

-- Donation amount must be non-negative
ALTER TABLE donations
    DROP CONSTRAINT IF EXISTS chk_donations_amount_non_negative;
ALTER TABLE donations
    ADD CONSTRAINT chk_donations_amount_non_negative
    CHECK (amount >= 0);

-- Revenue record amount must be non-negative
ALTER TABLE revenue_records
    DROP CONSTRAINT IF EXISTS chk_revenue_records_amount_non_negative;
ALTER TABLE revenue_records
    ADD CONSTRAINT chk_revenue_records_amount_non_negative
    CHECK (amount >= 0);

-- Aid request requested amount must be non-negative
ALTER TABLE aid_requests
    DROP CONSTRAINT IF EXISTS chk_aid_requests_requested_amount_non_negative;
ALTER TABLE aid_requests
    ADD CONSTRAINT chk_aid_requests_requested_amount_non_negative
    CHECK (requested_amount >= 0);

-- Aid request approved amount must be non-negative
ALTER TABLE aid_requests
    DROP CONSTRAINT IF EXISTS chk_aid_requests_approved_amount_non_negative;
ALTER TABLE aid_requests
    ADD CONSTRAINT chk_aid_requests_approved_amount_non_negative
    CHECK (approved_amount >= 0);

-- ============================================================================
-- QUERY SUPPORT INDEX
-- audit_logs is append-only and grows monotonically; the list endpoint and
-- report queries paginate with ORDER BY created_at DESC, so this ordering
-- index is justified by the existing query pattern.
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at);
