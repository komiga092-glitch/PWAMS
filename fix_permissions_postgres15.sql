-- PWAMS PostgreSQL Permission Fix for SQLSTATE 3F000
-- 
-- IMPORTANT: This script MUST be run as a PostgreSQL superuser (usually 'postgres')
--
-- HOW TO RUN:
-- 
-- Option 1: Using psql command line
--   psql -U postgres -d pwams_db -f fix_permissions_postgres15.sql
--
-- Option 2: Using Docker (if using docker-compose)
--   docker compose exec -T postgres psql -U postgres -d pwams_db -f /tmp/fix_permissions_postgres15.sql
--
-- Option 3: Using pgAdmin or another GUI tool
--   1. Connect as 'postgres' superuser to 'pwams_db' database
--   2. Open SQL editor
--   3. Paste and run the commands below
--
-- Option 4: Command line with inline commands
--   psql -U postgres -d pwams_db -c "GRANT USAGE ON SCHEMA public TO pwams_user;"
--   psql -U postgres -d pwams_db -c "GRANT CREATE ON SCHEMA public TO pwams_user;"
--   psql -U postgres -d pwams_db -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO pwams_user;"
--   psql -U postgres -d pwams_db -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO pwams_user;"
--   psql -U postgres -d pwams_db -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO pwams_user;"
--   psql -U postgres -d pwams_db -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO pwams_user;"

-- Verify the user exists
SELECT usename FROM pg_user WHERE usename = 'pwams_user';

-- Grant USAGE privilege (allows user to access the schema)
GRANT USAGE ON SCHEMA public TO pwams_user;

-- Grant CREATE privilege (allows user to create tables in the schema)
GRANT CREATE ON SCHEMA public TO pwams_user;

-- Grant privileges on all existing tables (for future migrations that might need to modify tables)
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO pwams_user;

-- Grant privileges on all existing sequences
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO pwams_user;

-- Set default privileges for future objects created by any user
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO pwams_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO pwams_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO pwams_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TYPES TO pwams_user;

-- Verify the grants were applied
SELECT 
    has_schema_privilege('pwams_user', 'public', 'USAGE') AS has_usage,
    has_schema_privilege('pwams_user', 'public', 'CREATE') AS has_create;

-- List all schemas and their owners
SELECT schema_name, schema_owner 
FROM information_schema.schemata 
WHERE schema_name = 'public';