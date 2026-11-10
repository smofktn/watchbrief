-- +goose Up
-- Retain a small tombstone after Collection deletion: an old retry must never recreate it.
CREATE TABLE app.collection_creation_requests (
    user_id text COLLATE "C" NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
    request_key text COLLATE "C" NOT NULL CHECK (request_key ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
    input_digest bytea NOT NULL CHECK (octet_length(input_digest) = 32),
    collection_id text COLLATE "C" UNIQUE REFERENCES app.collections(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, request_key)
);
GRANT SELECT, INSERT ON app.collection_creation_requests TO watchbrief_api;
-- enabled and timestamps are DB defaults; no general UPDATE/DELETE or child writes.
GRANT INSERT (id, user_id, name, research_instruction, initial_lookback_mode, initial_lookback_days, display_order)
ON app.collections TO watchbrief_api;

-- +goose Down
-- Removes retry records; never a normal startup/recovery operation.
REVOKE INSERT (id, user_id, name, research_instruction, initial_lookback_mode, initial_lookback_days, display_order)
ON app.collections FROM watchbrief_api;
DROP TABLE app.collection_creation_requests;
