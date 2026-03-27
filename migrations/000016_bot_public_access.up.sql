ALTER TABLE entities
    ADD COLUMN IF NOT EXISTS require_access_password BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS access_password_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_access_password_chk;

ALTER TABLE entities
    ADD CONSTRAINT entities_access_password_chk
    CHECK (
        (require_access_password = FALSE)
        OR (discoverability = 'external_public' AND access_password_hash <> '')
    );

CREATE TABLE IF NOT EXISTS bot_access_links (
    id BIGSERIAL PRIMARY KEY,
    bot_entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    code VARCHAR(64) NOT NULL UNIQUE,
    label TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NULL,
    max_uses INTEGER NOT NULL DEFAULT 0 CHECK (max_uses >= 0),
    used_count INTEGER NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    created_by_entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS bot_access_links_bot_idx
    ON bot_access_links (bot_entity_id, created_at DESC);
