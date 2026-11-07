-- +goose Up
CREATE TABLE app.collections (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    user_id text COLLATE "C" NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
    name text NOT NULL CHECK (length(name) > 0),
    research_instruction text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT false,
    initial_lookback_mode text NOT NULL DEFAULT 'ai' CHECK (initial_lookback_mode IN ('ai', 'days')),
    initial_lookback_days integer,
    display_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((initial_lookback_mode = 'ai' AND initial_lookback_days IS NULL)
        OR (initial_lookback_mode = 'days' AND initial_lookback_days BETWEEN 1 AND 365 AND initial_lookback_days IS NOT NULL))
);
CREATE TABLE app.collection_schedules (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    collection_id text COLLATE "C" NOT NULL UNIQUE REFERENCES app.collections(id) ON DELETE CASCADE,
    cron_expression text NOT NULL CHECK (length(cron_expression) > 0),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE app.watch_targets (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    collection_id text COLLATE "C" NOT NULL REFERENCES app.collections(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) > 0),
    instruction text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT false,
    display_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE app.information_sources (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    watch_target_id text COLLATE "C" NOT NULL REFERENCES app.watch_targets(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) > 0),
    url text NOT NULL CHECK (length(url) > 0),
    instruction text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX collections_user_order_idx ON app.collections(user_id, display_order, id);
CREATE INDEX watch_targets_collection_order_idx ON app.watch_targets(collection_id, display_order, id);
CREATE INDEX information_sources_target_idx ON app.information_sources(watch_target_id, id);
GRANT SELECT ON app.collections, app.collection_schedules, app.watch_targets, app.information_sources TO watchbrief_api;

-- +goose Down
-- Destructive: removes all current Collection settings. Never part of normal startup.
DROP TABLE app.information_sources;
DROP TABLE app.watch_targets;
DROP TABLE app.collection_schedules;
DROP TABLE app.collections;
