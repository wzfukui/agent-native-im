DROP TABLE IF EXISTS bot_access_links;

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_access_password_chk;

ALTER TABLE entities
    DROP COLUMN IF EXISTS access_password_hash,
    DROP COLUMN IF EXISTS require_access_password;
