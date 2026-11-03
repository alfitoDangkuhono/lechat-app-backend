DROP TRIGGER IF EXISTS trg_messages_direct_contact ON messages;
DROP TRIGGER IF EXISTS trg_contacts_direct_membership ON conversations_members;

DROP FUNCTION IF EXISTS enforce_direct_message_contact();
DROP FUNCTION IF EXISTS enforce_direct_contact_membership();

DROP TABLE IF EXISTS contacts;
