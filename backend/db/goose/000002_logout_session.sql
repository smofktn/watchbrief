-- +goose Up
GRANT DELETE ON app.auth_sessions TO watchbrief_api;

-- +goose Down
REVOKE DELETE ON app.auth_sessions FROM watchbrief_api;
