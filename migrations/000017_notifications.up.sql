CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    recipient_entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    actor_entity_id BIGINT REFERENCES entities(id) ON DELETE SET NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unread',
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS notifications_recipient_status_created_idx
    ON notifications (recipient_entity_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS notifications_recipient_created_idx
    ON notifications (recipient_entity_id, created_at DESC);
