-- +goose Up
CREATE TABLE app.users (
    id text COLLATE "C" PRIMARY KEY
        CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    login_id text COLLATE "C" NOT NULL UNIQUE
        CHECK (login_id ~ '^[a-z0-9_-]{1,64}$'),
    password_hash text NOT NULL CHECK (octet_length(password_hash) > 0),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE app.auth_sessions (
    id text COLLATE "C" PRIMARY KEY
        CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    user_id text COLLATE "C" NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32)
);

GRANT SELECT ON app.users TO watchbrief_api;
GRANT SELECT, INSERT ON app.auth_sessions TO watchbrief_api;
GRANT INSERT ON app.users TO watchbrief_user_creator;

-- +goose Down
-- Destructive: this removes authentication data. Not part of normal local startup.
DROP TABLE app.auth_sessions;
DROP TABLE app.users;
