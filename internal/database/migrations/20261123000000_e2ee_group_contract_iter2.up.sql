ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS key_version INT NULL;

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS chk_messages_key_version;

ALTER TABLE messages
    ADD CONSTRAINT chk_messages_key_version
    CHECK (key_version IS NULL OR key_version >= 1);

CREATE TABLE IF NOT EXISTS conversation_group_keys (
    id BIGSERIAL PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    key_version INT NOT NULL DEFAULT 1,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sender_id BIGINT NOT NULL REFERENCES users(id),
    encrypted_key TEXT NOT NULL,
    iv TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_group_key_per_user UNIQUE (conversation_id, key_version, user_id)
);

CREATE INDEX IF NOT EXISTS idx_group_keys_lookup 
    ON conversation_group_keys(conversation_id, user_id, key_version);
