-- name: CreateSession :exec
INSERT INTO sessions (session_id, user_id, token_data, id_token, expires_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW());

-- name: GetSession :one
SELECT session_id, user_id, token_data, id_token, expires_at, created_at, updated_at
FROM sessions
WHERE session_id = $1
LIMIT 1;

-- name: UpdateSessionToken :exec
UPDATE sessions
SET token_data = $2,
    updated_at = NOW()
WHERE session_id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE session_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < NOW();
