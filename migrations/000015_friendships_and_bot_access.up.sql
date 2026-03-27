ALTER TABLE entities
    ADD COLUMN IF NOT EXISTS discoverability VARCHAR(32) NOT NULL DEFAULT 'private',
    ADD COLUMN IF NOT EXISTS allow_non_friend_chat BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_discoverability_chk;

ALTER TABLE entities
    ADD CONSTRAINT entities_discoverability_chk
    CHECK (discoverability IN ('private', 'platform_public', 'external_public'));

CREATE TABLE IF NOT EXISTS friend_requests (
    id BIGSERIAL PRIMARY KEY,
    source_entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    message TEXT NOT NULL DEFAULT '',
    resolved_by BIGINT REFERENCES entities(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (source_entity_id <> target_entity_id),
    CHECK (status IN ('pending', 'accepted', 'rejected', 'canceled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS friend_requests_pending_unique_idx
    ON friend_requests (source_entity_id, target_entity_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS friend_requests_target_status_idx
    ON friend_requests (target_entity_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS friend_requests_source_status_idx
    ON friend_requests (source_entity_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS friendships (
    id BIGSERIAL PRIMARY KEY,
    entity_low_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    entity_high_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    created_by BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (entity_low_id < entity_high_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS friendships_pair_unique_idx
    ON friendships (entity_low_id, entity_high_id);

CREATE INDEX IF NOT EXISTS friendships_low_idx
    ON friendships (entity_low_id, created_at DESC);

CREATE INDEX IF NOT EXISTS friendships_high_idx
    ON friendships (entity_high_id, created_at DESC);
