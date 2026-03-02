-- name: UpsertUser :exec
INSERT INTO users (id, name, email, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5,  $5)
ON CONFLICT (id)
DO UPDATE SET
    name         = EXCLUDED.name,
    email        = EXCLUDED.email,
    role         = EXCLUDED.role,
    updated_at   = NOW();

-- name: GetUserByID :one
SELECT id, name, email,  role, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByEmail :one
SELECT id, name, email,  role, created_at, updated_at
FROM users
WHERE email = $1
LIMIT 1;
