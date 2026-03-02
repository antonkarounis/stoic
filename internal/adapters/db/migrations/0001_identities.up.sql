CREATE TABLE identities (
    id            BIGSERIAL    PRIMARY KEY,
    auth_sub      TEXT         NOT NULL UNIQUE,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
