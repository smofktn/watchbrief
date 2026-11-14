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
