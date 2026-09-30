-- Rollback for 000007 is intentionally a no-op.
--
-- The reconciliation merges the orphan Partner role into the canonical
-- Manager role and is recorded in audit_logs. There is no safe automatic
-- reverse: re-creating a "Partner" role would reintroduce the duplicate
-- Manager concept the project removed. Restoring requires an explicit,
-- operator-approved decision (re-create the role row and re-point the
-- affected soft-deleted accounts by ID from the audit record).
SELECT 1;
