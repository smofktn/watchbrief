-- name: CreateSession :exec
INSERT INTO app.auth_sessions (id, user_id, token_hash) VALUES ($1, $2, $3);

-- name: UserBySessionHash :one
SELECT u.login_id FROM app.auth_sessions s
JOIN app.users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: DeleteSessionByHash :exec
DELETE FROM app.auth_sessions WHERE token_hash = $1;
