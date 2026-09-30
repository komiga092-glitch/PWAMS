-- PHASE 4F — Partner -> Manager reconciliation (migration 000007).
--
-- The canonical schema contains BOTH a Manager role and an orphan Partner
-- role, so the blind rename in 000005 (UPDATE roles SET name='Manager'
-- WHERE name='Partner') would violate the unique role-name index and must
-- never be re-run as-is. This migration is the idempotent, fail-closed
-- reconciliation:
--
--   * No Partner role            -> no-op.
--   * Partner role, no Manager   -> rename in place (original 000005
--     semantics): every user keeps its role_id, no collision possible.
--   * Partner role AND Manager   -> collision risk. DETERMINISTIC RULE:
--     any LIVE user (deleted_at IS NULL) still holding Partner makes the
--     situation ambiguous (assigning them to Manager could breach the
--     one-active-Manager rule and would silently change a live user's
--     role) -> FAIL CLOSED. The operator must resolve the assignment
--     explicitly first. When only soft-deleted accounts hold Partner,
--     their role_id is re-pointed to the canonical Manager (they remain
--     soft-deleted; no live role changes) and the orphan role row is
--     removed (metadata cleanup, no user account is deleted).
--
-- Never touches the legacy `public` schema. Idempotent: once reconciled,
-- no Partner role exists and the migration becomes a no-op.

DO $$
DECLARE
    manager_role_id UUID;
    partner_role_id UUID;
    partner_user_count INT;
    live_partner_users INT;
BEGIN
    SELECT id INTO manager_role_id FROM roles WHERE name = 'Manager' LIMIT 1;
    SELECT id INTO partner_role_id FROM roles WHERE name = 'Partner' LIMIT 1;

    IF partner_role_id IS NULL THEN
        RAISE NOTICE '000007: no Partner role present; nothing to reconcile';

        -- Auditability: record the no-op reconciliation so every execution
        -- path leaves a trail (guarded for idempotent re-runs).
        INSERT INTO audit_logs (id, action, entity, details, created_at)
        SELECT gen_random_uuid(),
               'migration',
               'role',
               '000007: no Partner role present; nothing to reconcile',
               NOW()
        WHERE NOT EXISTS (
            SELECT 1 FROM audit_logs
            WHERE action = 'migration' AND details LIKE '000007:%'
        );
        RETURN;
    END IF;

    SELECT count(*) INTO partner_user_count FROM users WHERE role_id = partner_role_id;

    IF manager_role_id IS NULL THEN
        -- No canonical Manager exists: the rename cannot collide.
        UPDATE roles SET name = 'Manager' WHERE id = partner_role_id;
        RAISE NOTICE '000007: renamed Partner role % to Manager (% users keep their role_id)',
            partner_role_id, partner_user_count;

        INSERT INTO audit_logs (id, action, entity, details, created_at)
        VALUES (
            gen_random_uuid(),
            'migration',
            'role',
            format('000007: renamed legacy Partner role into Manager (rename path, %s users keep their role_id)', partner_user_count),
            NOW()
        );
        RETURN;
    END IF;

    SELECT count(*) INTO live_partner_users
    FROM users
    WHERE role_id = partner_role_id
      AND deleted_at IS NULL;

    IF live_partner_users > 0 THEN
        RAISE EXCEPTION '000007 fail-closed: % live user(s) still hold the legacy Partner role while a Manager role exists; resolve the assignment explicitly (assign them to an organization/role) before migrating',
            live_partner_users;
    END IF;

    -- Only soft-deleted accounts reference the orphan role: re-point them
    -- to the canonical Manager role (they stay soft-deleted; no live user
    -- changes role), then remove the orphan role row.
    UPDATE users SET role_id = manager_role_id WHERE role_id = partner_role_id;

    DELETE FROM roles WHERE id = partner_role_id;

    INSERT INTO audit_logs (id, action, entity, details, created_at)
    SELECT gen_random_uuid(),
           'migration',
           'role',
           format('000007: merged legacy Partner role %s into canonical Manager role; %s soft-deleted account(s) re-pointed; 0 live users changed', partner_role_id, partner_user_count),
           NOW()
    WHERE NOT EXISTS (
        SELECT 1 FROM audit_logs
        WHERE action = 'migration' AND details LIKE '000007:%'
    );

    RAISE NOTICE '000007: Partner role merged into Manager (% soft-deleted accounts re-pointed)', partner_user_count;
END $$;
