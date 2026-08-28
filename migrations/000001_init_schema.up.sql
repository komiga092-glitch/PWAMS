-- PWAMS Schema Migration v1
-- Run with: migrate -path migrations -database "$DATABASE_URL" up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Roles
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role_id UUID NOT NULL REFERENCES roles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'Active',
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- Sessions
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);

-- Persons
CREATE TABLE IF NOT EXISTS persons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(150) NOT NULL,
    nic_passport VARCHAR(30) NOT NULL UNIQUE,
    date_of_birth TIMESTAMPTZ,
    gender VARCHAR(20),
    phone VARCHAR(20),
    email VARCHAR(100),
    address TEXT,
    occupation VARCHAR(100),
    monthly_income NUMERIC(12,2) DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'Active',
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_persons_full_name ON persons(full_name);
CREATE INDEX IF NOT EXISTS idx_persons_status ON persons(status);
CREATE INDEX IF NOT EXISTS idx_persons_created_by_id ON persons(created_by_id);
CREATE INDEX IF NOT EXISTS idx_persons_deleted_at ON persons(deleted_at);
CREATE INDEX IF NOT EXISTS idx_persons_is_deleted ON persons(is_deleted);
CREATE INDEX IF NOT EXISTS idx_persons_tenant_id ON persons(tenant_id);

-- Students
CREATE TABLE IF NOT EXISTS students (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES persons(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    school_name VARCHAR(200),
    grade VARCHAR(50),
    status VARCHAR(20) NOT NULL DEFAULT 'Active',
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_students_person_id ON students(person_id);
CREATE INDEX IF NOT EXISTS idx_students_deleted_at ON students(deleted_at);
CREATE INDEX IF NOT EXISTS idx_students_tenant_id ON students(tenant_id);

-- Donors
CREATE TABLE IF NOT EXISTS donors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID REFERENCES persons(id) ON UPDATE CASCADE ON DELETE SET NULL,
    donor_type VARCHAR(50) NOT NULL DEFAULT 'Individual',
    name VARCHAR(200) NOT NULL,
    contact_person VARCHAR(200),
    phone VARCHAR(20),
    email VARCHAR(100),
    address TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'Active',
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_donors_deleted_at ON donors(deleted_at);
CREATE INDEX IF NOT EXISTS idx_donors_tenant_id ON donors(tenant_id);

-- Donations
CREATE TABLE IF NOT EXISTS donations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    donor_id UUID NOT NULL REFERENCES donors(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    person_id UUID REFERENCES persons(id) ON UPDATE CASCADE ON DELETE SET NULL,
    donation_type VARCHAR(50) NOT NULL,
    amount NUMERIC(14,2) DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'LKR',
    item_name VARCHAR(150),
    quantity NUMERIC(12,2) DEFAULT 0,
    unit VARCHAR(30),
    description TEXT,
    donation_date TIMESTAMPTZ NOT NULL,
    reference_no VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'Pending',
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_donations_donor_id ON donations(donor_id);
CREATE INDEX IF NOT EXISTS idx_donations_donation_type ON donations(donation_type);
CREATE INDEX IF NOT EXISTS idx_donations_status ON donations(status);
CREATE INDEX IF NOT EXISTS idx_donations_deleted_at ON donations(deleted_at);
CREATE INDEX IF NOT EXISTS idx_donations_tenant_id ON donations(tenant_id);

-- Aid Requests
CREATE TABLE IF NOT EXISTS aid_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES persons(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    request_type VARCHAR(50) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'Pending',
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_aid_requests_person_id ON aid_requests(person_id);
CREATE INDEX IF NOT EXISTS idx_aid_requests_deleted_at ON aid_requests(deleted_at);
CREATE INDEX IF NOT EXISTS idx_aid_requests_tenant_id ON aid_requests(tenant_id);

-- Care Provided
CREATE TABLE IF NOT EXISTS care_provided (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES persons(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    care_type VARCHAR(50) NOT NULL,
    description TEXT,
    care_date TIMESTAMPTZ NOT NULL,
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_care_provided_person_id ON care_provided(person_id);
CREATE INDEX IF NOT EXISTS idx_care_provided_deleted_at ON care_provided(deleted_at);
CREATE INDEX IF NOT EXISTS idx_care_provided_tenant_id ON care_provided(tenant_id);

-- Loans
CREATE TABLE IF NOT EXISTS loans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    person_id UUID NOT NULL REFERENCES persons(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    loan_amount NUMERIC(15,2) NOT NULL,
    interest_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
    duration_months INT NOT NULL,
    installment_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    status VARCHAR(30) NOT NULL DEFAULT 'Pending',
    purpose TEXT,
    approved_by_id UUID,
    approved_at TIMESTAMPTZ,
    disbursed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_loans_person_id ON loans(person_id);
CREATE INDEX IF NOT EXISTS idx_loans_status ON loans(status);
CREATE INDEX IF NOT EXISTS idx_loans_is_deleted ON loans(is_deleted);
CREATE INDEX IF NOT EXISTS idx_loans_tenant_id ON loans(tenant_id);

-- Loan Repayments
CREATE TABLE IF NOT EXISTS loan_repayments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    loan_id UUID NOT NULL REFERENCES loans(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    amount NUMERIC(15,2) NOT NULL,
    payment_date TIMESTAMPTZ NOT NULL,
    reference_no VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'Pending',
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_loan_repayments_loan_id ON loan_repayments(loan_id);
CREATE INDEX IF NOT EXISTS idx_loan_repayments_is_deleted ON loan_repayments(is_deleted);
CREATE INDEX IF NOT EXISTS idx_loan_repayments_tenant_id ON loan_repayments(tenant_id);

-- Revenue Records
CREATE TABLE IF NOT EXISTS revenue_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    record_type VARCHAR(20) NOT NULL,
    category VARCHAR(50) NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'LKR',
    record_date TIMESTAMPTZ NOT NULL,
    description TEXT,
    reference_no VARCHAR(100),
    created_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id UUID
);
CREATE INDEX IF NOT EXISTS idx_revenue_records_record_type ON revenue_records(record_type);
CREATE INDEX IF NOT EXISTS idx_revenue_records_category ON revenue_records(category);
CREATE INDEX IF NOT EXISTS idx_revenue_records_deleted_at ON revenue_records(deleted_at);
CREATE INDEX IF NOT EXISTS idx_revenue_records_tenant_id ON revenue_records(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_revenue_reference ON revenue_records(reference_no) WHERE reference_no <> '';

-- Audit Logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    entity VARCHAR(100) NOT NULL,
    entity_id UUID,
    details TEXT,
    old_value TEXT,
    new_value TEXT,
    request_id VARCHAR(100),
    ip_address VARCHAR(45),
    tenant_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_id ON audit_logs(tenant_id);

-- Notifications
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);

-- Messages
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    recipient_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    subject VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_recipient_id ON messages(recipient_id);

-- File Uploads
CREATE TABLE IF NOT EXISTS file_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    idempotency_key VARCHAR(255),
    original_name VARCHAR(500) NOT NULL,
    stored_name VARCHAR(500) NOT NULL,
    path VARCHAR(1000) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    sha256 VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_file_uploads_user_id ON file_uploads(user_id);
CREATE INDEX IF NOT EXISTS idx_file_uploads_deleted_at ON file_uploads(deleted_at);

-- Password Reset Tokens
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);

-- Account Activation Tokens
CREATE TABLE IF NOT EXISTS account_activation_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_account_activation_tokens_user_id ON account_activation_tokens(user_id);

-- Sync Idempotency Records
CREATE TABLE IF NOT EXISTS sync_idempotency_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    request_hash VARCHAR(64) NOT NULL,
    response_body TEXT NOT NULL,
    status_code INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sync_idempotency_key ON sync_idempotency_records(idempotency_key);
