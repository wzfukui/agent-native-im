-- Message recall support
ALTER TABLE messages ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- @mention support
ALTER TABLE messages ADD COLUMN IF NOT EXISTS mentions JSONB NOT NULL DEFAULT '[]';

-- Participant subscription mode
ALTER TABLE participants ADD COLUMN IF NOT EXISTS subscription_mode VARCHAR(32) NOT NULL DEFAULT 'mention_only';

-- Allow bootstrap credential type
ALTER TABLE credentials DROP CONSTRAINT IF EXISTS credentials_cred_type_check;
ALTER TABLE credentials ADD CONSTRAINT credentials_cred_type_check CHECK (cred_type IN ('password', 'api_key', 'oauth', 'bootstrap'));
