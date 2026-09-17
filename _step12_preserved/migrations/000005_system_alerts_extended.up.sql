-- 000005_system_alerts_extended.up.sql
-- Professional system-alert architecture.
--
-- Adds stable alert codes, categories, severity (including CRITICAL), source,
-- safe metadata, and occurrence-count deduplication to the system_alerts
-- table created in 000004. Existing rows are preserved; all new columns are
-- additive with safe defaults.
--
-- Deduplication model: repeated identical failures for the same open alert
-- (same alert_code + source, still 'open') increment occurrence_count and
-- refresh last_occurred_at instead of inserting a new row. The partial
-- unique index below makes the dedup find-or-update race-safe at the
-- database level, mirroring the existing pending deletion-request partial
-- unique index convention (see migrate.go).

ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS alert_code VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS category VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS title_key VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS severity VARCHAR(20) NOT NULL DEFAULT 'error';
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS source VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS safe_metadata TEXT;
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS occurrence_count INT NOT NULL DEFAULT 1;
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS first_occurred_at TIMESTAMPTZ;
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS last_occurred_at TIMESTAMPTZ;
ALTER TABLE system_alerts ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;

-- Backfill lifecycle timestamps from created_at so pre-existing alerts have
-- consistent first/last occurrence values (idempotent: only fills NULLs).
UPDATE system_alerts
   SET first_occurred_at = created_at
 WHERE first_occurred_at IS NULL;
UPDATE system_alerts
   SET last_occurred_at = created_at
 WHERE last_occurred_at IS NULL;

-- Backfill severity from the legacy level column so pre-existing alerts are
-- visible to the new severity-based UI (idempotent: only fills empty values).
UPDATE system_alerts
   SET severity = level
 WHERE severity = '';

-- Backfill alert_code from message_key and source from presentation so
-- legacy rows participate in the dedup/grouping semantics.
UPDATE system_alerts
   SET alert_code = message_key
 WHERE alert_code = '';
UPDATE system_alerts
   SET source = presentation
 WHERE source = '';

-- Race-safe dedup: while a given (alert_code, source) alert is still open,
-- only one row may exist. The service upserts on this key.
CREATE UNIQUE INDEX IF NOT EXISTS uq_system_alerts_open_code_source
    ON system_alerts (alert_code, source) WHERE status = 'open';

-- Improved query indexes for the professional alert inbox.
CREATE INDEX IF NOT EXISTS idx_system_alerts_severity ON system_alerts (severity);
CREATE INDEX IF NOT EXISTS idx_system_alerts_category ON system_alerts (category);
CREATE INDEX IF NOT EXISTS idx_system_alerts_source ON system_alerts (source);
CREATE INDEX IF NOT EXISTS idx_system_alerts_last_occurred_at ON system_alerts (last_occurred_at DESC);