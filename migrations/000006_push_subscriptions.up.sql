CREATE TABLE push_subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    entity_id  BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    device_id  VARCHAR(255) NOT NULL DEFAULT '',
    endpoint   TEXT NOT NULL,
    key_p256dh TEXT NOT NULL,
    key_auth   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, endpoint)
);

CREATE INDEX idx_push_subs_entity ON push_subscriptions(entity_id);
