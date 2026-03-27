DROP TABLE IF EXISTS friendships;
DROP TABLE IF EXISTS friend_requests;

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_discoverability_chk;

ALTER TABLE entities
    DROP COLUMN IF EXISTS allow_non_friend_chat,
    DROP COLUMN IF EXISTS discoverability;
