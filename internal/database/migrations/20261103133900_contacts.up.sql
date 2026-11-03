CREATE TABLE contacts (
    user_low BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_high BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_by BIGINT NOT NULL REFERENCES users(id),
    status VARCHAR NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    accepted_at TIMESTAMPTZ NULL,
    PRIMARY KEY (user_low, user_high),
    CONSTRAINT chk_contacts_user_pair CHECK (user_low < user_high),
    CONSTRAINT chk_contacts_requester CHECK (requested_by IN (user_low, user_high)),
    CONSTRAINT chk_contacts_acceptance_timestamp CHECK (
        (status = 'pending' AND accepted_at IS NULL)
        OR (status = 'accepted' AND accepted_at IS NOT NULL)
    )
);

CREATE INDEX idx_contacts_user_low_status ON contacts (user_low, status);
CREATE INDEX idx_contacts_user_high_status ON contacts (user_high, status);

CREATE FUNCTION enforce_direct_contact_membership()
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

    IF conversation_type = 'direct' THEN
        IF NEW.user_id NOT IN (direct_user_low, direct_user_high) THEN
            RAISE EXCEPTION 'direct conversation members must match its participant pair';
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM contacts
            WHERE user_low = direct_user_low
              AND user_high = direct_user_high
              AND status = 'accepted'
        ) THEN
            RAISE EXCEPTION 'direct conversations require accepted contacts';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_contacts_direct_membership
BEFORE INSERT OR UPDATE OF conversation_id, user_id ON conversations_members
FOR EACH ROW
EXECUTE FUNCTION enforce_direct_contact_membership();

CREATE FUNCTION enforce_direct_message_contact()
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
       AND NOT EXISTS (
            SELECT 1
            FROM contacts
            WHERE user_low = direct_user_low
              AND user_high = direct_user_high
              AND status = 'accepted'
       ) THEN
        RAISE EXCEPTION 'direct messages require accepted contacts';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_messages_direct_contact
BEFORE INSERT OR UPDATE OF conversation_id ON messages
FOR EACH ROW
EXECUTE FUNCTION enforce_direct_message_contact();
