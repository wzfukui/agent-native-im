-- Agent-Native IM v2 — Initial Schema
-- Unified entity model, participant-based conversations, rich message types

-- 1. Unified identity (replaces users + bots)
CREATE TABLE entities (
    id           BIGSERIAL PRIMARY KEY,
    entity_type  VARCHAR(16) NOT NULL CHECK (entity_type IN ('user', 'bot', 'service')),
    name         VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url   VARCHAR(1024) NOT NULL DEFAULT '',
    status       VARCHAR(32) NOT NULL DEFAULT 'active',
    metadata     JSONB NOT NULL DEFAULT '{}',
    owner_id     BIGINT REFERENCES entities(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_entities_name_type ON entities(name, entity_type);

-- 2. Credentials (password / API key / OAuth)
CREATE TABLE credentials (
    id          BIGSERIAL PRIMARY KEY,
    entity_id   BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    cred_type   VARCHAR(32) NOT NULL CHECK (cred_type IN ('password', 'api_key', 'oauth')),
    secret_hash VARCHAR(255) NOT NULL,
    raw_prefix  VARCHAR(8) NOT NULL DEFAULT '',
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_credentials_entity ON credentials(entity_id);
CREATE INDEX idx_credentials_lookup ON credentials(cred_type, raw_prefix);

-- 3. Conversations (direct / group / channel)
CREATE TABLE conversations (
    id         BIGSERIAL PRIMARY KEY,
    conv_type  VARCHAR(16) NOT NULL DEFAULT 'direct' CHECK (conv_type IN ('direct', 'group', 'channel')),
    title      VARCHAR(500) NOT NULL DEFAULT '',
    metadata   JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Participants (n:m entity <-> conversation)
CREATE TABLE participants (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    entity_id       BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    role            VARCHAR(32) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member', 'observer')),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at         TIMESTAMPTZ,
    UNIQUE(conversation_id, entity_id)
);

CREATE INDEX idx_participants_entity ON participants(entity_id);
CREATE INDEX idx_participants_conv ON participants(conversation_id);

-- 5. Messages (with content_type + attachments)
CREATE TABLE messages (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       BIGINT NOT NULL REFERENCES entities(id),
    stream_id       VARCHAR(64) NOT NULL DEFAULT '',
    content_type    VARCHAR(32) NOT NULL DEFAULT 'text',
    layers          JSONB NOT NULL DEFAULT '{}',
    attachments     JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_conv ON messages(conversation_id, id DESC);
CREATE INDEX idx_messages_sender ON messages(sender_id);

-- 6. Webhooks (decoupled from entity, supports event filtering)
CREATE TABLE webhooks (
    id         BIGSERIAL PRIMARY KEY,
    entity_id  BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    url        VARCHAR(2048) NOT NULL,
    secret     VARCHAR(255) NOT NULL DEFAULT '',
    events     TEXT[] NOT NULL DEFAULT '{"message.new"}',
    status     VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_entity ON webhooks(entity_id);
