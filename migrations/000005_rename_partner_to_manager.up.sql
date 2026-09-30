-- The SRS specifies an NGO-level "Manager" role (one active Manager per
-- NGO). The role was originally seeded as "Partner". Rename it in place so
-- existing users keep their role assignment (role_id is unchanged).
--
-- PHASE 4P baseline-safety (project convention, see migrations/embed.go:
-- every migration must be idempotent so applying it to a database whose
-- structure was previously managed by GORM AutoMigrate is always safe):
-- the canonical pwams_user schema already contains BOTH a Manager role and
-- an orphan Partner role, so the original unguarded UPDATE would violate
-- the unique role-name constraint. The rename therefore only fires when no
-- Manager role exists (the original rename scenario); when a Manager
-- already exists, the orphan Partner role is left for the dedicated,
-- fail-closed reconciliation in 000007. No user row is touched.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM roles WHERE name = 'Partner') THEN
        IF EXISTS (SELECT 1 FROM roles WHERE name = 'Manager') THEN
            RAISE NOTICE '000005: Manager role already exists; orphan Partner role left for 000007 reconciliation';
        ELSE
            UPDATE roles SET name = 'Manager' WHERE name = 'Partner';
            RAISE NOTICE '000005: renamed Partner role to Manager (no Manager existed; users keep their role_id)';
        END IF;
    ELSE
        RAISE NOTICE '000005: no Partner role present; nothing to rename';
    END IF;
END $$;
