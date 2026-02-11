CREATE TABLE users (
    id            BIGSERIAL    PRIMARY KEY,
    auth_sub      TEXT         NOT NULL UNIQUE,
    email         TEXT         NOT NULL,
    display_name  TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
