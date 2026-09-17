-- PWAMS Schema Migration v3: managed-account profile fields (Step 2).
-- Run with: migrate -path migrations -database "$DATABASE_URL" up
--
-- Additive only: adds nullable-with-default profile columns to users so the
-- Admin/Partner/Staff/Volunteer management forms (Step 2) can store the
-- full name, phone number and preferred language. No existing column is
-- modified and no row is touched, so this is safe for existing data.
ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(30) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS language VARCHAR(30) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_full_name ON users(full_name);
