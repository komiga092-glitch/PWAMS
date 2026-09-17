-- PWAMS Schema Migration v3 rollback: drop the Step 2 profile columns.
-- Safe: only removes the columns added by 000003_user_profile.up.sql.
DROP INDEX IF EXISTS idx_users_full_name;
ALTER TABLE users DROP COLUMN IF EXISTS language;
ALTER TABLE users DROP COLUMN IF EXISTS phone;
ALTER TABLE users DROP COLUMN IF EXISTS full_name;
