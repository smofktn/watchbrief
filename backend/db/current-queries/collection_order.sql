-- name: LockCollectionOrder :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(user_id)::text, 18));

-- name: CollectionOrderRequest :one
SELECT input_digest FROM app.collection_order_requests
WHERE user_id = $1 AND request_key = $2;

-- name: ListOwnedCollectionOrder :many
SELECT id, version, display_order FROM app.collections
WHERE user_id = $1
ORDER BY display_order, id
FOR UPDATE;

-- name: UpdateOwnedCollectionOrder :execrows
UPDATE app.collections AS c
SET display_order = (desired.position - 1)::integer,
    version = c.version + 1,
    updated_at = clock_timestamp()
FROM unnest(sqlc.arg(ordered_ids)::text[]) WITH ORDINALITY AS desired(id, position)
WHERE c.id = desired.id
  AND c.user_id = sqlc.arg(user_id)
  AND c.display_order <> (desired.position - 1)::integer;

-- name: RecordCollectionOrderRequest :exec
INSERT INTO app.collection_order_requests(user_id, request_key, input_digest)
VALUES ($1, $2, $3);
