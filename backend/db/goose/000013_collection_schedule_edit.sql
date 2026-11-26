-- +goose Up
GRANT INSERT (id, collection_id, cron_expression), UPDATE (cron_expression, updated_at)
ON app.collection_schedules TO watchbrief_api;

-- +goose Down
REVOKE INSERT (id, collection_id, cron_expression), UPDATE (cron_expression, updated_at)
ON app.collection_schedules FROM watchbrief_api;
