-- DAY1 / PHASE 2 — RBAC gate: admin deletion approval workflow.
-- The application also AutoMigrates this table; this migration keeps
-- external `migrate`-tool deployments in sync.

CREATE TABLE IF NOT EXISTS admin_deletion_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    requested_by_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'Pending',
    approved_by_id UUID,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_admin_deletion_requests_target ON admin_deletion_requests(target_user_id);
CREATE INDEX IF NOT EXISTS idx_admin_deletion_requests_requested_by ON admin_deletion_requests(requested_by_id);
CREATE INDEX IF NOT EXISTS idx_admin_deletion_requests_status ON admin_deletion_requests(status);
CREATE INDEX IF NOT EXISTS idx_admin_deletion_requests_deleted_at ON admin_deletion_requests(deleted_at);