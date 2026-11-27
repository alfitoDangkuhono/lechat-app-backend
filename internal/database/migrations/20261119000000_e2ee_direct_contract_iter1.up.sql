ALTER TABLE users
    ADD COLUMN IF NOT EXISTS public_key_updated_at TIMESTAMPTZ NULL;

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS chk_messages_encrypted_payload_consistency,
    DROP CONSTRAINT IF EXISTS chk_messages_encryption_version;

ALTER TABLE messages
    ADD CONSTRAINT chk_messages_encrypted_payload_consistency
    CHECK (
        (encryption_version IS NULL AND ciphertext IS NULL AND iv IS NULL)
        OR
        (encryption_version IS NOT NULL AND ciphertext IS NOT NULL AND iv IS NOT NULL)
    ),
    ADD CONSTRAINT chk_messages_encryption_version
    CHECK (encryption_version IS NULL OR encryption_version = 1);

CREATE TABLE IF NOT EXISTS messages_acks (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    acked_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_messages_acks_user ON messages_acks (user_id, acked_at);
