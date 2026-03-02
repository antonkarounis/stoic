CREATE TABLE sessions (
    session_id   TEXT         PRIMARY KEY,
    identity_id  BIGINT       NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    token_data   JSONB        NOT NULL,
    id_token     TEXT         NOT NULL,
    expires_at   TIMESTAMPTZ  NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_identity_id ON sessions(identity_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
