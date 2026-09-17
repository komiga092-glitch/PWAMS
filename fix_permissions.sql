-- Run this file as a PostgreSQL superuser (like postgres) to fix the schema permissions
-- Command: psql -U postgres -d pwams_db -f fix_permissions.sql

-- Grant usage and create privileges on the public schema to pwams_user
GRANT USAGE ON SCHEMA public TO pwams_user;
GRANT CREATE ON SCHEMA public TO pwams_user;

-- Grant default privileges for future objects
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO pwams_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO pwams_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO pwams_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TYPES TO pwams_user;

-- Alternative: Create a dedicated schema for the application
-- CREATE SCHEMA IF NOT EXISTS pwams_schema;
-- GRANT ALL ON SCHEMA pwams_schema TO pwams_user;
-- ALTER USER pwams_user SET search_path TO pwams_schema;

-- Verify the changes
\dn+ public