-- 000006_system_alerts_level_sync.up.sql
-- Defense-in-depth fix for the legacy level NOT NULL constraint.
--
-- Root cause: migration 000004 introduced system_alerts.level as NOT NULL with
-- no DEFAULT. Migration 000005 added the new severity column and backfilled
-- existing rows (severity = level), but the Go model only populates severity.
-- New INSERTs therefore leave level NULL, violating the NOT NULL constraint.
--
-- The Go model now keeps level in sync with severity via BeforeCreate/BeforeSave
-- hooks. This migration adds a database-level trigger as defense-in-depth so
-- the constraint is satisfied even if application code paths bypass the hooks.
--
-- Existing rows are preserved. The trigger only fires when level is NULL, so
-- it never overwrites a legitimate value.

-- Backfill any legacy rows where level is NULL (defensive; should be a no-op
-- given the NOT NULL constraint, but guards against partially-applied states).
UPDATE system_alerts
   SET level = COALESCE(NULLIF(severity, ''), 'error')
 WHERE level IS NULL;

-- Trigger function: ensure level is populated on every INSERT/UPDATE.
CREATE OR REPLACE FUNCTION trg_system_alerts_set_level()
RETURNS TRIGGER AS $$
BEGIN
    -- Keep level synchronised with severity. If severity is blank, fall back
    -- to 'error' (the application default). This guarantees the NOT NULL
    -- constraint on level is always satisfied.
    IF NEW.level IS NULL OR NEW.level = '' THEN
        NEW.level := COALESCE(NULLIF(NEW.severity, ''), 'error');
    END IF;
    -- Also ensure severity stays in sync with level (defense-in-depth for
    -- direct DB writes that populate only level).
    IF NEW.severity IS NULL OR NEW.severity = '' THEN
        NEW.severity := COALESCE(NULLIF(NEW.level, ''), 'error');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Attach the trigger. Idempotent: drop any prior version first.
DROP TRIGGER IF EXISTS trg_system_alerts_set_level ON system_alerts;
CREATE TRIGGER trg_system_alerts_set_level
    BEFORE INSERT OR UPDATE ON system_alerts
    FOR EACH ROW
    EXECUTE FUNCTION trg_system_alerts_set_level();

-- Defense-in-depth: add a column-level DEFAULT so even a raw INSERT that
-- omits both level and severity succeeds. The application still provides
-- the value explicitly; this is a last-resort safety net.
ALTER TABLE system_alerts ALTER COLUMN level SET DEFAULT 'error';
