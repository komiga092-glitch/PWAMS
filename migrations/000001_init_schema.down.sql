-- Rollback: PWAMS Schema Migration v1

DROP TABLE IF EXISTS sync_idempotency_records;
DROP TABLE IF EXISTS account_activation_tokens;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS file_uploads;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS revenue_records;
DROP TABLE IF EXISTS loan_repayments;
DROP TABLE IF EXISTS loans;
DROP TABLE IF EXISTS care_provided;
DROP TABLE IF EXISTS aid_requests;
DROP TABLE IF EXISTS donations;
DROP TABLE IF EXISTS donors;
DROP TABLE IF EXISTS students;
DROP TABLE IF EXISTS persons;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
DROP EXTENSION IF EXISTS "pgcrypto";
