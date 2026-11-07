-- +goose Up
CREATE INDEX auth_sessions_user_id_idx ON app.auth_sessions (user_id);

-- +goose Down
DROP INDEX app.auth_sessions_user_id_idx;
