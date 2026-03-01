DROP INDEX IF EXISTS idx_messages_summary_search;
ALTER TABLE messages DROP COLUMN IF EXISTS reply_to;
