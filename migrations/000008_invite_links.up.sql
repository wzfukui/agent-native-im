CREATE TABLE IF NOT EXISTS invite_links (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    code            VARCHAR(32) NOT NULL UNIQUE,
    created_by      BIGINT NOT NULL REFERENCES entities(id),
    max_uses        INT NOT NULL DEFAULT 0,
    use_count       INT NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_invite_code ON invite_links(code);
