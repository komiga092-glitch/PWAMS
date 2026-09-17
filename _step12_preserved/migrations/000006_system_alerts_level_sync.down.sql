-- 000006_system_alerts_level_sync.down.sql
-- Reverts the defense-in-depth trigger and default added in 000006.
--
-- Note: the Go model's BeforeCreate/BeforeSave hooks still keep level in sync
-- with severity, so rolling back this migration does NOT reintroduce the
-- original bug. The application-level fix is sufficient on its own.

DROP TRIGGER IF EXISTS trg_system_alerts_set_level ON system_alerts;
DROP FUNCTION IF EXISTS trg_system_alerts_set_level();

ALTER TABLE system_alerts ALTER COLUMN level DROP DEFAULT;
