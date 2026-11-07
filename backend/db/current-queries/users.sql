-- name: CreateUser :exec
INSERT INTO app.users (id, login_id, password_hash) VALUES ($1, $2, $3);

-- name: UserByLoginID :one
SELECT id, login_id, password_hash FROM app.users WHERE login_id = $1;

-- name: LockUserCredentials :exec
SELECT pg_advisory_xact_lock(hashtextextended('watchbrief.credentials:' || sqlc.arg(user_id)::text, 0));

-- name: UserPasswordHash :one
SELECT password_hash FROM app.users WHERE id = $1;

-- name: ResetUserPassword :execrows
UPDATE app.users SET password_hash = $2, updated_at = clock_timestamp() WHERE id = $1;

-- name: UserIDByLoginID :one
SELECT id FROM app.users WHERE login_id = $1;
