CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL,
    recipient_id UUID NOT NULL,
    subject VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMPTZ,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT fk_messages_sender
        FOREIGN KEY (sender_id)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,
    CONSTRAINT fk_messages_recipient
        FOREIGN KEY (recipient_id)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_messages_recipient_created_at
    ON messages(recipient_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_messages_sender_created_at
    ON messages(sender_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_messages_recipient_unread
    ON messages(recipient_id, is_read)
    WHERE is_read = FALSE;

CREATE INDEX IF NOT EXISTS idx_messages_created_at
    ON messages(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_messages_deleted_at
    ON messages(deleted_at);