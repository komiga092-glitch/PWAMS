-- 000004_system_alerts.up.sql
-- System alert inbox: records server-side errors (panics, 5xx responses)
-- for privileged operators (Super Admin by default). Technical details are
-- stored so critical context is available to the privileged audience only.

CREATE TABLE IF NOT EXISTS system_alerts (
    id                 UUID PRIMARY KEY,
    level              VARCHAR(20)  NOT NULL,
    message_key        VARCHAR(200) NOT NULL,
    presentation       VARCHAR(100) NOT NULL DEFAULT '',
    technical_details  TEXT,
    status             VARCHAR(20)  NOT NULL DEFAULT 'open',
    resolved_by        UUID,
    resolved_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_alerts_created_at ON system_alerts (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_system_alerts_status ON system_alerts (status);
CREATE INDEX IF NOT EXISTS idx_system_alerts_level ON system_alerts (level);