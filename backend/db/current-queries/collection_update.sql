-- name: CollectionUpdateState :one
SELECT version, enabled FROM app.collections WHERE id = $1 AND user_id = $2;

-- name: CollectionUpdateRequest :one
SELECT collection_id, input_digest, result_version FROM app.collection_update_requests
WHERE user_id = $1 AND request_key = $2;

-- name: UpdateOwnedCollection :one
UPDATE app.collections SET name = $3, research_instruction = $4,
    initial_lookback_mode = $5, initial_lookback_days = $6,
    version = version + 1, updated_at = clock_timestamp()
WHERE id = $1 AND user_id = $2 AND version = $7 AND enabled = false
RETURNING version;

-- name: RecordCollectionUpdate :exec
INSERT INTO app.collection_update_requests(user_id, request_key, collection_id, input_digest, result_version)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateCollectionTarget :one
UPDATE app.watch_targets SET name = $3, instruction = $4, updated_at = clock_timestamp()
WHERE id = $1 AND collection_id = $2
RETURNING id;

-- name: AppendCollectionTarget :exec
INSERT INTO app.watch_targets(id, collection_id, name, instruction, enabled, display_order)
SELECT $1, $2, $3, $4, true, COALESCE(MAX(display_order), -1) + 1
FROM app.watch_targets WHERE collection_id = $2;

-- name: CollectionSourceTarget :one
SELECT id FROM app.watch_targets WHERE id = $1 AND collection_id = $2;

-- name: AppendCollectionSource :exec
INSERT INTO app.information_sources(id, watch_target_id, name, url, instruction)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateCollectionSource :one
UPDATE app.information_sources SET name = $3, url = $4, instruction = $5, updated_at = clock_timestamp()
WHERE id = $1 AND watch_target_id = $2
RETURNING id;

-- name: CollectionScheduleID :one
SELECT id FROM app.collection_schedules WHERE collection_id = $1;

-- name: AppendCollectionSchedule :exec
INSERT INTO app.collection_schedules(id, collection_id, cron_expression) VALUES ($1, $2, $3);

-- name: UpdateCollectionSchedule :one
UPDATE app.collection_schedules SET cron_expression = $3, updated_at = clock_timestamp()
WHERE id = $1 AND collection_id = $2 RETURNING id;
