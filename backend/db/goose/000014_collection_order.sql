-- +goose Up
CREATE TABLE app.collection_order_requests (
    user_id text COLLATE "C" NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
    request_key text COLLATE "C" NOT NULL CHECK (request_key ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
    input_digest bytea NOT NULL CHECK (octet_length(input_digest) = 32),
    PRIMARY KEY (user_id, request_key)
);
GRANT SELECT, INSERT ON app.collection_order_requests TO watchbrief_api;
GRANT UPDATE (display_order) ON app.collections TO watchbrief_api;

-- +goose Down
REVOKE UPDATE (display_order) ON app.collections FROM watchbrief_api;
DROP TABLE app.collection_order_requests;
