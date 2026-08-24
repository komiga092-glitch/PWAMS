ALTER TABLE persons
    ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS updated_by UUID,
    ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS tenant_id UUID;

CREATE INDEX IF NOT EXISTS idx_persons_sync_updated_at
    ON persons(updated_at);

CREATE INDEX IF NOT EXISTS idx_persons_sync_is_deleted
    ON persons(is_deleted);

CREATE INDEX IF NOT EXISTS idx_persons_sync_tenant_id
    ON persons(tenant_id);