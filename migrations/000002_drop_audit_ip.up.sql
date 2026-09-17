-- STEP 15.6.4: Remove IP address from audit logs.
-- Audit logs must never record IP addresses (PII).
ALTER TABLE audit_logs DROP COLUMN IF EXISTS ip_address;
