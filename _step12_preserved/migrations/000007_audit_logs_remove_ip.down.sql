-- Rollback: re-add ip_address column (kept nullable so existing rows are unaffected).

ALTER TABLE IF EXISTS audit_logs ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45);
