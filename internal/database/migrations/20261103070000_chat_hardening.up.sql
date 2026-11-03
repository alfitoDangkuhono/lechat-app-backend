ALTER TABLE conversations
    ADD COLUMN direct_user_low BIGINT REFERENCES users(id),
    ADD COLUMN direct_user_high BIGINT REFERENCES users(id);

UPDATE conversations AS c
SET direct_user_low = participants.user_low,
    direct_user_high = participants.user_high
FROM (
    SELECT conversation_id, MIN(user_id) AS user_low, MAX(user_id) AS user_high
    FROM conversations_members
    GROUP BY conversation_id
    HAVING COUNT(*) = 2
) AS participants
WHERE c.id = participants.conversation_id
  AND c.type = 'direct';

ALTER TABLE conversations
    ADD CONSTRAINT chk_conversations_direct_pair
    CHECK (
        (type = 'direct'
            AND direct_user_low IS NOT NULL
            AND direct_user_high IS NOT NULL
            AND direct_user_low < direct_user_high)
        OR
        (type = 'group'
            AND direct_user_low IS NULL
            AND direct_user_high IS NULL)
    );

CREATE UNIQUE INDEX idx_conversations_direct_pair
    ON conversations (direct_user_low, direct_user_high)
    WHERE type = 'direct';

CREATE FUNCTION enforce_direct_conversation_member_pair()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    conversation_type VARCHAR;
    direct_user_low BIGINT;
    direct_user_high BIGINT;
BEGIN
    SELECT type, c.direct_user_low, c.direct_user_high
    INTO conversation_type, direct_user_low, direct_user_high
    FROM conversations AS c
    WHERE c.id = NEW.conversation_id;

    IF conversation_type = 'direct'
       AND NEW.user_id NOT IN (direct_user_low, direct_user_high) THEN
        RAISE EXCEPTION 'direct conversation members must match its participant pair';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_conversations_members_direct_pair
BEFORE INSERT OR UPDATE OF conversation_id, user_id ON conversations_members
FOR EACH ROW
EXECUTE FUNCTION enforce_direct_conversation_member_pair();

ALTER TABLE messages
    ADD CONSTRAINT uq_messages_sender_client_id
    UNIQUE (sender_id, client_message_id),
    ADD CONSTRAINT fk_messages_conversation_sender
    FOREIGN KEY (conversation_id, sender_id)
    REFERENCES conversations_members (conversation_id, user_id);

CREATE INDEX idx_messages_conversation_timeline
    ON messages (conversation_id, created_at DESC, id DESC);

CREATE INDEX idx_messages_receipts_user_unread
    ON messages_receipts (user_id, read_at);

CREATE TABLE message_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    storage_key VARCHAR NOT NULL UNIQUE,
    mime_type VARCHAR NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    original_filename VARCHAR NULL,
    sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (message_id, sort_order)
);

CREATE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_conversations_updated_at
BEFORE UPDATE ON conversations
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_messages_updated_at
BEFORE UPDATE ON messages
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE FUNCTION enforce_active_message_sender()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM 1
    FROM conversations_members
    WHERE conversation_id = NEW.conversation_id
      AND user_id = NEW.sender_id
      AND left_at IS NULL
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'message sender must be an active conversation member';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_messages_active_sender
BEFORE INSERT OR UPDATE OF conversation_id, sender_id ON messages
FOR EACH ROW
EXECUTE FUNCTION enforce_active_message_sender();

CREATE FUNCTION enforce_message_attachment_type()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM messages
        WHERE id = NEW.message_id
          AND type IN ('image', 'video', 'file')
    ) THEN
        RAISE EXCEPTION 'attachments require an image, video, or file message';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_message_attachments_type
BEFORE INSERT OR UPDATE OF message_id ON message_attachments
FOR EACH ROW
EXECUTE FUNCTION enforce_message_attachment_type();

CREATE FUNCTION prevent_text_message_with_attachments()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.type = 'text'
       AND EXISTS (
           SELECT 1
           FROM message_attachments
           WHERE message_id = NEW.id
       ) THEN
        RAISE EXCEPTION 'text messages cannot have attachments';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_messages_attachment_type
BEFORE UPDATE OF type ON messages
FOR EACH ROW
EXECUTE FUNCTION prevent_text_message_with_attachments();
