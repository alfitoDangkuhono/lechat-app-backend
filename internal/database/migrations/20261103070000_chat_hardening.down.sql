DROP TRIGGER IF EXISTS trg_message_attachments_type ON message_attachments;
DROP TRIGGER IF EXISTS trg_messages_attachment_type ON messages;
DROP TRIGGER IF EXISTS trg_messages_active_sender ON messages;
DROP TRIGGER IF EXISTS trg_messages_updated_at ON messages;
DROP TRIGGER IF EXISTS trg_conversations_updated_at ON conversations;
DROP TRIGGER IF EXISTS trg_conversations_members_direct_pair ON conversations_members;

DROP TABLE IF EXISTS message_attachments;

DROP FUNCTION IF EXISTS prevent_text_message_with_attachments();
DROP FUNCTION IF EXISTS enforce_message_attachment_type();
DROP FUNCTION IF EXISTS enforce_active_message_sender();
DROP FUNCTION IF EXISTS enforce_direct_conversation_member_pair();
DROP FUNCTION IF EXISTS set_updated_at();

DROP INDEX IF EXISTS idx_messages_receipts_user_unread;
DROP INDEX IF EXISTS idx_messages_conversation_timeline;
DROP INDEX IF EXISTS idx_conversations_direct_pair;

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS fk_messages_conversation_sender,
    DROP CONSTRAINT IF EXISTS uq_messages_sender_client_id;

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS chk_conversations_direct_pair,
    DROP COLUMN IF EXISTS direct_user_high,
    DROP COLUMN IF EXISTS direct_user_low;
