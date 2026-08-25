ALTER TABLE file_uploads
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(100);

CREATE UNIQUE INDEX IF NOT EXISTS idx_file_uploads_user_idempotency
    ON file_uploads(user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';