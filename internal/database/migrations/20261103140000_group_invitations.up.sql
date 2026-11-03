CREATE TABLE group_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    inviter_id BIGINT NOT NULL REFERENCES users(id),
    invitee_id BIGINT NOT NULL REFERENCES users(id),
    status VARCHAR NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    responded_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_group_invitation_distinct_users CHECK (inviter_id <> invitee_id),
    CONSTRAINT chk_group_invitation_response_time CHECK (
        (status = 'pending' AND responded_at IS NULL)
        OR (status <> 'pending' AND responded_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_group_invitations_one_pending
    ON group_invitations (conversation_id, invitee_id)
    WHERE status = 'pending';

CREATE INDEX idx_group_invitations_incoming
    ON group_invitations (invitee_id, created_at DESC)
    WHERE status = 'pending';

CREATE TRIGGER trg_group_invitations_updated_at
BEFORE UPDATE ON group_invitations
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
