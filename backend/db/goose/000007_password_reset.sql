-- +goose Up
-- Provision this login role separately before running migrations.
GRANT SELECT (id, login_id) ON app.users TO watchbrief_password_resetter;
GRANT UPDATE (password_hash, updated_at) ON app.users TO watchbrief_password_resetter;
GRANT SELECT (user_id), DELETE ON app.auth_sessions TO watchbrief_password_resetter;

-- +goose Down
REVOKE SELECT (id, login_id), UPDATE (password_hash, updated_at) ON app.users FROM watchbrief_password_resetter;
REVOKE SELECT (user_id), DELETE ON app.auth_sessions FROM watchbrief_password_resetter;
