-- +goose Up
GRANT INSERT (id, collection_id, name, instruction, enabled, display_order)
ON app.watch_targets TO watchbrief_api;
GRANT UPDATE (name, instruction, updated_at) ON app.watch_targets TO watchbrief_api;

-- +goose Down
REVOKE INSERT (id, collection_id, name, instruction, enabled, display_order)
ON app.watch_targets FROM watchbrief_api;
REVOKE UPDATE (name, instruction, updated_at) ON app.watch_targets FROM watchbrief_api;
