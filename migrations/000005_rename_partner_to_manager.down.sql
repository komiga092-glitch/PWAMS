-- Symmetric, guarded rollback: only reverses a rename that actually
-- happened (Manager exists, Partner absent, and no user holds Manager).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM roles WHERE name = 'Manager')
       AND NOT EXISTS (SELECT 1 FROM roles WHERE name = 'Partner')
       AND NOT EXISTS (
           SELECT 1 FROM users u JOIN roles r ON r.id = u.role_id
           WHERE r.name = 'Manager'
       )
    THEN
        UPDATE roles SET name = 'Partner' WHERE name = 'Manager';
    ELSE
        RAISE NOTICE '000005 down: Manager role in use or Partner already exists; rollback skipped';
    END IF;
END $$;
