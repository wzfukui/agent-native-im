ALTER TABLE messages DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE messages DROP COLUMN IF EXISTS mentions;
ALTER TABLE participants DROP COLUMN IF EXISTS subscription_mode;
