CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE entities
    ADD COLUMN IF NOT EXISTS public_id UUID,
    ADD COLUMN IF NOT EXISTS bot_id VARCHAR(128);

UPDATE entities
SET public_id = NULLIF(metadata->>'public_id', '')::uuid
WHERE public_id IS NULL
  AND NULLIF(metadata->>'public_id', '') IS NOT NULL;

UPDATE entities
SET public_id = gen_random_uuid()
WHERE public_id IS NULL;

ALTER TABLE entities
    ALTER COLUMN public_id SET NOT NULL;

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_bot_id_format,
    ADD CONSTRAINT entities_bot_id_format
        CHECK (
            bot_id IS NULL
            OR bot_id ~ '^bot_[a-z0-9][a-z0-9_-]{2,63}$'
        );

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_bot_id_entity_type,
    ADD CONSTRAINT entities_bot_id_entity_type
        CHECK (
            bot_id IS NULL
            OR entity_type = 'bot'
        );

CREATE UNIQUE INDEX IF NOT EXISTS idx_entities_public_id ON entities(public_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_entities_bot_id_lower ON entities ((lower(bot_id))) WHERE bot_id IS NOT NULL;
