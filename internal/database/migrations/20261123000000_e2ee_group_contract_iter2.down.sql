DROP TABLE IF EXISTS conversation_group_keys;

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS chk_messages_key_version;

ALTER TABLE messages
    DROP COLUMN IF EXISTS key_version;
