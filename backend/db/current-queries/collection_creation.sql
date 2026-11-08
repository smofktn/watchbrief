-- name: LockCollectionCreation :exec
-- Separate namespace from credential locks; all creates of one owner serialize.
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(user_id)::text, 18));

-- name: CollectionCreationRequest :one
SELECT input_digest, collection_id FROM app.collection_creation_requests
WHERE user_id = $1 AND request_key = $2;

-- name: AppendOwnedCollection :exec
INSERT INTO app.collections (id,user_id,name,research_instruction,initial_lookback_mode,initial_lookback_days,display_order)
SELECT sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(research_instruction),
sqlc.arg(initial_lookback_mode), sqlc.narg(initial_lookback_days)::integer,
COALESCE(MAX(display_order)::bigint + 1, 0)::integer
FROM app.collections WHERE user_id = sqlc.arg(user_id);

-- name: RecordCollectionCreation :exec
INSERT INTO app.collection_creation_requests(user_id,request_key,input_digest,collection_id)
VALUES ($1,$2,$3,$4);
