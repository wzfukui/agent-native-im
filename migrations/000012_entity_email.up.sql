-- Add email column to entities table for username/email login
ALTER TABLE entities ADD COLUMN IF NOT EXISTS email VARCHAR(255) DEFAULT '';

-- Unique index on email (only for non-empty values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_entities_email_unique
    ON entities (email) WHERE email != '';
