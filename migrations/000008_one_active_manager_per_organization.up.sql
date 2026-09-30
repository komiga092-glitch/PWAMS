-- PHASE 4F — one active Manager per organization (migration 000008).
--
-- Invariant: (organization_id, Manager role, Active status) has at most
-- one row per organization.
--
-- Why a trigger + advisory lock instead of a plain unique partial index:
-- "Manager" lives in roles.name (another table) and a partial unique
-- index predicate must be immutable — it cannot contain a subquery. The
-- trigger serialises conflicting writes per organization scope with
-- pg_advisory_xact_lock; a concurrent second assignment blocks on the
-- lock and, under READ COMMITTED, re-evaluates the existence check AFTER
-- the first transaction commits — so two concurrent assignments can
-- never both succeed.
--
-- Scope: rows WITH organization_id set. Platform-level rows
-- (organization_id IS NULL — the current single-tenant deployment state,
-- including the existing active Manager user) keep the Phase 2
-- service-level global rule and are deliberately NOT constrained here;
-- constraining them would change live Phase 2/3 behaviour.
--
-- Idempotent: CREATE OR REPLACE FUNCTION + DROP/CREATE TRIGGER.

CREATE OR REPLACE FUNCTION enforce_one_active_manager() RETURNS trigger AS $$
DECLARE
    target_is_manager BOOLEAN;
    active_manager_count INT;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM roles WHERE roles.id = NEW.role_id AND roles.name = 'Manager'
    ) INTO target_is_manager;

    IF NEW.organization_id IS NULL
       OR NOT target_is_manager
       OR NEW.status <> 'Active'
       OR NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    -- Serialise per organization scope so concurrent assignments cannot
    -- race past the existence check.
    PERFORM pg_advisory_xact_lock(
        hashtextextended('pwams_one_active_manager:' || NEW.organization_id::text, 0)
    );

    SELECT count(*) INTO active_manager_count
    FROM users u
    JOIN roles r ON r.id = u.role_id
    WHERE r.name = 'Manager'
      AND u.id <> NEW.id
      AND u.status = 'Active'
      AND u.deleted_at IS NULL
      AND u.organization_id = NEW.organization_id;

    IF active_manager_count > 0 THEN
        RAISE EXCEPTION
            'one active Manager per organization: organization % already has an active Manager',
            NEW.organization_id
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_one_active_manager ON users;

CREATE TRIGGER trg_users_one_active_manager
BEFORE INSERT OR UPDATE OF role_id, organization_id, status, deleted_at ON users
FOR EACH ROW EXECUTE FUNCTION enforce_one_active_manager();
