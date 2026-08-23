CREATE TABLE IF NOT EXISTS revenue_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    record_type VARCHAR(20) NOT NULL CHECK (record_type IN ('income', 'expense')),
    category VARCHAR(50) NOT NULL CHECK (category IN ('Donations', 'Loan Repayments', 'Grants', 'Administrative Expenses', 'Welfare Expenses')),
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'LKR',
    record_date TIMESTAMPTZ NOT NULL,
    description TEXT,
    reference_no VARCHAR(100),
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_revenue_records_type ON revenue_records(record_type);
CREATE INDEX IF NOT EXISTS idx_revenue_records_category ON revenue_records(category);
CREATE INDEX IF NOT EXISTS idx_revenue_records_date ON revenue_records(record_date);
CREATE INDEX IF NOT EXISTS idx_revenue_records_deleted_at ON revenue_records(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_revenue_reference ON revenue_records(reference_no) WHERE reference_no <> '';
