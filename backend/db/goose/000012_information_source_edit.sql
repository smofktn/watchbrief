-- +goose Up
GRANT INSERT (id, watch_target_id, name, url, instruction) ON app.information_sources TO watchbrief_api;
GRANT UPDATE (name, url, instruction, updated_at) ON app.information_sources TO watchbrief_api;

-- +goose Down
REVOKE INSERT (id, watch_target_id, name, url, instruction) ON app.information_sources FROM watchbrief_api;
REVOKE UPDATE (name, url, instruction, updated_at) ON app.information_sources FROM watchbrief_api;
