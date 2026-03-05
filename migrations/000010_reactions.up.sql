CREATE TABLE reactions (
    id         BIGSERIAL    PRIMARY KEY,
    message_id BIGINT       NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    entity_id  BIGINT       NOT NULL REFERENCES entities(id),
    emoji      VARCHAR(32)  NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(message_id, entity_id, emoji)
);

CREATE INDEX idx_reactions_message ON reactions(message_id);
