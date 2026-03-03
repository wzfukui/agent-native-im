-- Conversation prompt field (agent context / system prompt)
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS prompt TEXT NOT NULL DEFAULT '';

-- Tasks table (per-conversation task management)
CREATE TABLE IF NOT EXISTS tasks (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    title           VARCHAR(500) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    assignee_id     BIGINT REFERENCES entities(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority        VARCHAR(10) NOT NULL DEFAULT 'medium',
    due_date        TIMESTAMPTZ,
    parent_task_id  BIGINT REFERENCES tasks(id) ON DELETE SET NULL,
    sort_order      INT NOT NULL DEFAULT 0,
    created_by      BIGINT NOT NULL REFERENCES entities(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_tasks_conv ON tasks(conversation_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee_id) WHERE assignee_id IS NOT NULL;

-- Conversation memories table
CREATE TABLE IF NOT EXISTS conversation_memories (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    key             VARCHAR(100) NOT NULL,
    content         TEXT NOT NULL,
    updated_by      BIGINT NOT NULL REFERENCES entities(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(conversation_id, key)
);

-- Conversation change requests table
CREATE TABLE IF NOT EXISTS conversation_change_requests (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    field           VARCHAR(50) NOT NULL,
    old_value       TEXT,
    new_value       TEXT NOT NULL,
    requester_id    BIGINT NOT NULL REFERENCES entities(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    approver_id     BIGINT REFERENCES entities(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_change_requests_conv ON conversation_change_requests(conversation_id, status);

-- Audit logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    entity_id       BIGINT,
    action          VARCHAR(50) NOT NULL,
    resource_type   VARCHAR(30),
    resource_id     BIGINT,
    details         JSONB NOT NULL DEFAULT '{}',
    ip_address      VARCHAR(45),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_logs(entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id);
