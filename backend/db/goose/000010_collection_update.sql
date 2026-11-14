-- +goose Up
ALTER TABLE app.collections ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE TABLE app.collection_update_requests (
    user_id text COLLATE "C" NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
    request_key text COLLATE "C" NOT NULL CHECK (request_key ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
    collection_id text COLLATE "C" REFERENCES app.collections(id) ON DELETE SET NULL,
    input_digest bytea NOT NULL CHECK (octet_length(input_digest) = 32),
    result_version bigint NOT NULL CHECK (result_version > 1),
    PRIMARY KEY (user_id, request_key)
);
GRANT SELECT, INSERT ON app.collection_update_requests TO watchbrief_api;
GRANT UPDATE (name, research_instruction, initial_lookback_mode, initial_lookback_days, version, updated_at)
ON app.collections TO watchbrief_api;

-- +goose Down
REVOKE UPDATE (name, research_instruction, initial_lookback_mode, initial_lookback_days, version, updated_at)
ON app.collections FROM watchbrief_api;
DROP TABLE app.collection_update_requests;
ALTER TABLE app.collections DROP COLUMN version;
