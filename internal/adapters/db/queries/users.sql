-- name: UpsertUser :one
INSERT INTO users (auth_sub, email, display_name, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (auth_sub)
DO UPDATE SET
    email = EXCLUDED.email,
    display_name = EXCLUDED.display_name,
    updated_at = NOW()
RETURNING id, auth_sub, email, display_name, created_at, updated_at;

-- name: GetUserByAuthSub :one
SELECT id, auth_sub, email, display_name, created_at, updated_at
FROM users
WHERE auth_sub = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT id, auth_sub, email, display_name, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1;
