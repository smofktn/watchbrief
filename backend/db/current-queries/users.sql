-- name: CreateUser :exec
INSERT INTO app.users (id, login_id, password_hash) VALUES ($1, $2, $3);

-- name: UserByLoginID :one
SELECT id, login_id, password_hash FROM app.users WHERE login_id = $1;
