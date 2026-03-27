DROP INDEX IF EXISTS idx_entities_bot_id_lower;
DROP INDEX IF EXISTS idx_entities_public_id;

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_bot_id_entity_type,
    DROP CONSTRAINT IF EXISTS entities_bot_id_format;

ALTER TABLE entities
    DROP COLUMN IF EXISTS bot_id,
    DROP COLUMN IF EXISTS public_id;
