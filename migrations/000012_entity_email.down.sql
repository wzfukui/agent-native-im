DROP INDEX IF EXISTS idx_entities_email_unique;
ALTER TABLE entities DROP COLUMN IF EXISTS email;
