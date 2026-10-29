CREATE TABLE IF NOT EXISTS token_revocations (
    token_id VARCHAR PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_token_revocations_expires_at
    ON token_revocations (expires_at);
