-- name: UpsertIdentity :one
INSERT INTO identities (auth_sub, last_login_at, updated_at)
VALUES ($1, NOW(), NOW())
ON CONFLICT (auth_sub)
DO UPDATE SET
    last_login_at = NOW(),
    updated_at    = NOW()
RETURNING id, auth_sub, last_login_at, user_id, created_at, updated_at;

-- name: GetIdentityByID :one
SELECT id, auth_sub, last_login_at, user_id, created_at, updated_at
FROM identities
WHERE id = $1
LIMIT 1;

-- name: LinkIdentityToUser :exec
UPDATE identities SET user_id = $2, updated_at = NOW() WHERE id = $1;
