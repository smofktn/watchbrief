-- name: CreateUser :exec
INSERT INTO users (id, login_id, password_hash) VALUES ($1, $2, $3);

-- name: UserByLogin :one
SELECT id, login_id, password_hash FROM users WHERE login_id = $1;

-- name: CreateSession :exec
INSERT INTO auth_sessions (id, user_id, token_hash) VALUES ($1, $2, $3);

-- name: UserByToken :one
SELECT users.id, users.login_id FROM auth_sessions
JOIN users ON users.id = auth_sessions.user_id WHERE token_hash = $1;
