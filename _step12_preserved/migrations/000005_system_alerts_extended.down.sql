-- 000005_system_alerts_extended.down.sql
-- Backward-safe rollback: drop the extended columns and indexes introduced in
-- 000005 while preserving the base table from 000004.
DROP INDEX IF EXISTS uq_system_alerts_open_code_source;
DROP INDEX IF EXISTS idx_system_alerts_severity;
DROP INDEX IF EXISTS idx_system_alerts_category;
DROP INDEX IF EXISTS idx_system_alerts_source;
DROP INDEX IF EXISTS idx_system_alerts_last_occurred_at;

ALTER TABLE system_alerts DROP COLUMN IF EXISTS alert_code;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS category;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS title_key;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS severity;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS source;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS safe_metadata;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS occurrence_count;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS first_occurred_at;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS last_occurred_at;
ALTER TABLE system_alerts DROP COLUMN IF EXISTS read_at;