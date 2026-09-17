-- Migration 000007: Remove ip_address from audit_logs
-- IP address is no longer captured in audit records (privacy requirement).
-- The column is dropped safely; existing records remain intact otherwise.

ALTER TABLE IF EXISTS audit_logs DROP COLUMN IF EXISTS ip_address;
