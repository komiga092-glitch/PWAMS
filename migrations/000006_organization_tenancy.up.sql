-- PHASE 4F/4G/4H — Organizations + tenant ownership (migration 000006).
--
-- Targets the CANONICAL application schema. Every statement uses
-- unqualified names: they resolve through the connection search_path
-- ("$user", public -> `pwams_user` in the current deployment). The legacy
-- `public` schema is never referenced and never modified.
--
-- Design rules (PHASE 4F/4G):
--   * organizations is the tenant registry; organizations.id is the
--     tenant identity shared with the existing tenant_id columns.
--   * users.organization_id is NULLABLE. NULL means platform-level
--     account (Super Admin) or a legacy account that has not been
--     assigned yet.
--   * NO existing user is assigned to an organization by this migration
--     and NO default organization is fabricated. Bootstrap is an
--     explicit, audited operator/admin action taken AFTER this migration.
--   * The ten tenant-aware tables keep their tenant_id columns and gain
--     a foreign key to organizations(id) so tenant ownership can never
--     dangle. Existing rows keep their current tenant_id value (all NULL
--     in the canonical schema today — verified by the Phase 4A audit).
--   * Idempotent: safe on databases previously managed by GORM
--     AutoMigrate and on clean databases.

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(150) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'Active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);

-- Platform users (Super Admin) and not-yet-assigned users keep NULL.
ALTER TABLE users ADD COLUMN IF NOT EXISTS organization_id UUID;

CREATE INDEX IF NOT EXISTS idx_users_organization_id ON users(organization_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_users_organization'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT fk_users_organization
            FOREIGN KEY (organization_id)
            REFERENCES organizations(id)
            ON UPDATE CASCADE
            ON DELETE RESTRICT;
    END IF;
END $$;

-- Tenant ownership: every existing tenant-aware table references the
-- organization registry through tenant_id. Idempotent per-table guard.
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
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conname = 'fk_' || tbl || '_tenant_organization'
              AND conrelid = format('%I', tbl)::regclass
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I ADD CONSTRAINT fk_%s_tenant_organization ' ||
                'FOREIGN KEY (tenant_id) REFERENCES organizations(id) ' ||
                'ON UPDATE CASCADE ON DELETE RESTRICT',
                tbl, tbl
            );
        END IF;
    END LOOP;
END $$;

-- Auditability: record that tenancy scaffolding was created and that no
-- user was assigned to any organization by this migration.
INSERT INTO audit_logs (id, action, entity, details, created_at)
SELECT gen_random_uuid(),
       'migration',
       'organizations',
       '000006: organizations table, users.organization_id and tenant_id ownership constraints created; no organization assigned to any existing user (bootstrap is an explicit operator action)',
       NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM audit_logs
    WHERE action = 'migration' AND details LIKE '000006:%'
);
