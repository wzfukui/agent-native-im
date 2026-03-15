CREATE TABLE IF NOT EXISTS file_records (
    id BIGSERIAL PRIMARY KEY,
    stored_name VARCHAR(255) NOT NULL UNIQUE,
    original_name VARCHAR(500) NOT NULL DEFAULT '',
    mime_type VARCHAR(255) NOT NULL DEFAULT 'application/octet-stream',
    size BIGINT NOT NULL DEFAULT 0,
    uploader_id BIGINT NOT NULL REFERENCES entities(id),
    conversation_id BIGINT REFERENCES conversations(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_file_records_conversation ON file_records(conversation_id);
-- stored_name already has a UNIQUE constraint which creates an implicit index
