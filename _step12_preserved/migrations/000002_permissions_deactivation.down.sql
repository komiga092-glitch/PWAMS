-- Rollback: PWAMS Schema Migration v2

DROP TABLE IF EXISTS deactivation_requests;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;

-- Restore the NOT NULL constraint on password_hash (v1 state).
UPDATE users SET password_hash = '' WHERE password_hash IS NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;