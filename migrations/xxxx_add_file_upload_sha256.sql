ALTER TABLE file_uploads
    ADD COLUMN IF NOT EXISTS sha256 CHAR(64);

CREATE INDEX IF NOT EXISTS idx_file_uploads_sha256
    ON file_uploads(sha256);