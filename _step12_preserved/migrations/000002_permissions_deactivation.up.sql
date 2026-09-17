-- PWAMS Schema Migration v2: granular permissions + Super Admin deactivation workflow.
-- Run with: migrate -path migrations -database "$DATABASE_URL" up

-- Permissions catalog (resource.action convention)
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    category VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_permissions_category ON permissions(category);

-- Role default permission assignments
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON UPDATE CASCADE ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON UPDATE CASCADE ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);

-- User-specific permission overrides (granted = false revokes a role default)
CREATE TABLE IF NOT EXISTS user_permissions (
    user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON UPDATE CASCADE ON DELETE CASCADE,
    granted BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, permission_id)
);
CREATE INDEX IF NOT EXISTS idx_user_permissions_permission_id ON user_permissions(permission_id);

-- Supervised Super Admin deactivation requests
CREATE TABLE IF NOT EXISTS deactivation_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_user_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    requester_id UUID NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ,
    responded_by_id UUID REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    response TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_deactivation_requests_target_user_id ON deactivation_requests(target_user_id);
CREATE INDEX IF NOT EXISTS idx_deactivation_requests_requester_id ON deactivation_requests(requester_id);
CREATE INDEX IF NOT EXISTS idx_deactivation_requests_status ON deactivation_requests(status);

-- Nullable password hash: accounts are created pending and the user sets
-- their own password during activation.
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ALTER COLUMN status SET DEFAULT 'Active';