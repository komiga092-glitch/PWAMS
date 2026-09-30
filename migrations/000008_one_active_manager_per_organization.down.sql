-- Rollback for 000008: removes the per-organization Manager-uniqueness
-- enforcement. Only run on dedicated test databases or after an explicit,
-- backed-up operator decision.
DROP TRIGGER IF EXISTS trg_users_one_active_manager ON users;

DROP FUNCTION IF EXISTS enforce_one_active_manager();
