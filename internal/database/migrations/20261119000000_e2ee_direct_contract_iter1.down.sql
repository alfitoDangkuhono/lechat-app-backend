DROP TABLE IF EXISTS messages_acks;

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS chk_messages_encryption_version,
    DROP CONSTRAINT IF EXISTS chk_messages_encrypted_payload_consistency;

ALTER TABLE users
    DROP COLUMN IF EXISTS public_key_updated_at;
