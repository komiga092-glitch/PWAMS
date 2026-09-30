-- Rollback for 000006. DESTRUCTIVE BY DESIGN: it removes the tenancy
-- scaffolding INCLUDING any organization assignments recorded after the
-- migration ran. Only run on dedicated test databases or after an
-- explicit, backed-up operator decision. The 000006 audit marker row is
-- deliberately retained (audit history must never be deleted).
DO $$
DECLARE
    tbl TEXT;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
        'persons', 'students', 'donors', 'donations', 'aid_requests',
        'care_provided', 'loans', 'loan_repayments', 'revenue_records',
        'audit_logs'
    ]
    LOOP
        EXECUTE format(
            'ALTER TABLE %I DROP CONSTRAINT IF EXISTS fk_%s_tenant_organization',
            tbl, tbl
        );
    END LOOP;
END $$;

DROP INDEX IF EXISTS idx_users_organization_id;

ALTER TABLE users DROP COLUMN IF EXISTS organization_id;

DROP TABLE IF EXISTS organizations CASCADE;
