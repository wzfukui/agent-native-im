DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS conversation_change_requests;
DROP TABLE IF EXISTS conversation_memories;
DROP TABLE IF EXISTS tasks;
ALTER TABLE conversations DROP COLUMN IF EXISTS prompt;
