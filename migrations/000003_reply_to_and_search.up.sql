-- reply_to: message quoting/referencing
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to BIGINT REFERENCES messages(id);

-- Full-text search index on message summary layer
CREATE INDEX IF NOT EXISTS idx_messages_summary_search
  ON messages USING GIN (to_tsvector('simple', COALESCE(layers->>'summary', '')));
