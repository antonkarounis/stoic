CREATE TABLE users (
    id           TEXT         PRIMARY KEY,
    name         TEXT         NOT NULL,
    email        TEXT         NOT NULL UNIQUE,
    role         TEXT         NOT NULL CHECK (role IN ('owner', 'member')),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
