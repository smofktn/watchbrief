-- +goose Up
CREATE TABLE app.generation_jobs (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    user_id text COLLATE "C" NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
    collection_id text COLLATE "C" REFERENCES app.collections(id) ON DELETE SET NULL,
    collection_name text NOT NULL CHECK (length(collection_name) > 0),
    input_snapshot jsonb NOT NULL CHECK (jsonb_typeof(input_snapshot) = 'object'),
    scheduled_at timestamptz NOT NULL,
    queued_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('queued','running','succeeded','failed')),
    UNIQUE(collection_id, scheduled_at)
);
CREATE TABLE app.generation_attempts (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    generation_job_id text COLLATE "C" NOT NULL UNIQUE REFERENCES app.generation_jobs(id) ON DELETE RESTRICT,
    attempt_number integer NOT NULL CHECK (attempt_number = 1),
    started_at timestamptz NOT NULL,
    heartbeat_at timestamptz NOT NULL CHECK (heartbeat_at >= started_at),
    finished_at timestamptz,
    status text NOT NULL CHECK (status IN ('running','succeeded','failed')),
    failure_type text CHECK (failure_type IN ('timeout','worker_interrupted','ai_error','invalid_output','persistence_error','unexpected_error')),
    UNIQUE(generation_job_id, attempt_number),
    CHECK ((status = 'running' AND finished_at IS NULL AND failure_type IS NULL)
        OR (status = 'succeeded' AND finished_at IS NOT NULL AND failure_type IS NULL)
        OR (status = 'failed' AND finished_at IS NOT NULL AND failure_type IS NOT NULL)),
    CHECK (finished_at IS NULL OR finished_at >= heartbeat_at)
);
CREATE TABLE app.reports (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    generation_attempt_id text COLLATE "C" NOT NULL UNIQUE REFERENCES app.generation_attempts(id) ON DELETE RESTRICT,
    body_markdown text NOT NULL CHECK (length(body_markdown) > 0),
    summary_markdown text NOT NULL,
    updates_started_at timestamptz NOT NULL
);
CREATE TABLE app.report_target_results (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    report_id text COLLATE "C" NOT NULL REFERENCES app.reports(id) ON DELETE RESTRICT,
    target_id text COLLATE "C" NOT NULL CHECK (target_id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    target_name text NOT NULL CHECK (length(target_name) > 0),
    display_order integer NOT NULL,
    status text NOT NULL CHECK (length(status) > 0),
    ai_summary_markdown text NOT NULL,
    UNIQUE(report_id,target_id)
);
CREATE TABLE app.report_facts (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    report_target_result_id text COLLATE "C" NOT NULL REFERENCES app.report_target_results(id) ON DELETE RESTRICT,
    fact_kind text NOT NULL CHECK (fact_kind IN ('current_state','update')),
    content_markdown text NOT NULL CHECK (length(content_markdown) > 0),
    display_order integer NOT NULL
);
CREATE TABLE app.report_sources (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    report_target_result_id text COLLATE "C" NOT NULL REFERENCES app.report_target_results(id) ON DELETE RESTRICT,
    source_id text COLLATE "C" CHECK (source_id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    source_kind text NOT NULL CHECK (source_kind IN ('registered','discovered')),
    source_name text NOT NULL CHECK (length(source_name) > 0),
    source_url text NOT NULL CHECK (length(source_url) > 0),
    CHECK ((source_kind = 'registered' AND source_id IS NOT NULL) OR (source_kind = 'discovered' AND source_id IS NULL))
);
CREATE TABLE app.fact_sources (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    report_fact_id text COLLATE "C" NOT NULL REFERENCES app.report_facts(id) ON DELETE RESTRICT,
    report_source_id text COLLATE "C" NOT NULL REFERENCES app.report_sources(id) ON DELETE RESTRICT,
    UNIQUE(report_fact_id,report_source_id)
);
CREATE TABLE app.report_source_failures (
    id text COLLATE "C" PRIMARY KEY CHECK (id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    report_target_result_id text COLLATE "C" NOT NULL REFERENCES app.report_target_results(id) ON DELETE RESTRICT,
    source_id text COLLATE "C" CHECK (source_id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
    source_kind text NOT NULL CHECK (source_kind IN ('registered','discovered')),
    source_name text NOT NULL CHECK (length(source_name) > 0),
    source_url text NOT NULL CHECK (length(source_url) > 0),
    reason text NOT NULL CHECK (length(reason) > 0),
    CHECK ((source_kind = 'registered' AND source_id IS NOT NULL) OR (source_kind = 'discovered' AND source_id IS NULL))
);
CREATE INDEX generation_jobs_user_scheduled_idx ON app.generation_jobs(user_id,scheduled_at,id);
CREATE INDEX generation_jobs_status_scheduled_idx ON app.generation_jobs(status,scheduled_at,id);
CREATE INDEX generation_jobs_collection_status_idx ON app.generation_jobs(collection_id,status);
CREATE INDEX report_target_results_report_order_idx ON app.report_target_results(report_id,display_order,id);
CREATE INDEX report_facts_target_order_idx ON app.report_facts(report_target_result_id,display_order,id);
CREATE INDEX report_sources_target_idx ON app.report_sources(report_target_result_id,id);
CREATE INDEX report_source_failures_target_idx ON app.report_source_failures(report_target_result_id,id);
CREATE INDEX fact_sources_source_idx ON app.fact_sources(report_source_id);

-- Validate the final transaction state, allowing Report and success updates in either order.
-- +goose StatementBegin
CREATE FUNCTION app.check_generation_consistency() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE job_id text;
BEGIN
    IF TG_TABLE_NAME = 'generation_jobs' THEN job_id := COALESCE(NEW.id, OLD.id);
    ELSIF TG_TABLE_NAME = 'generation_attempts' THEN job_id := COALESCE(NEW.generation_job_id, OLD.generation_job_id);
    ELSE SELECT generation_job_id INTO job_id FROM app.generation_attempts WHERE id = COALESCE(NEW.generation_attempt_id, OLD.generation_attempt_id);
    END IF;
    IF EXISTS (SELECT 1 FROM app.generation_jobs j LEFT JOIN app.generation_attempts a ON a.generation_job_id = j.id
        LEFT JOIN app.reports r ON r.generation_attempt_id = a.id WHERE j.id = job_id AND (
          (j.status = 'queued' AND a.id IS NOT NULL) OR
          (j.status <> 'queued' AND (a.id IS NULL OR j.status <> a.status)) OR
          (a.status = 'succeeded' AND r.id IS NULL) OR
          (a.status <> 'succeeded' AND r.id IS NOT NULL) OR
          a.started_at < j.queued_at OR r.updates_started_at > a.started_at)) THEN
        RAISE EXCEPTION 'Generation state inconsistent' USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER generation_jobs_consistency AFTER INSERT OR UPDATE ON app.generation_jobs
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION app.check_generation_consistency();
CREATE CONSTRAINT TRIGGER generation_attempts_consistency AFTER INSERT OR UPDATE OR DELETE ON app.generation_attempts
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION app.check_generation_consistency();
CREATE CONSTRAINT TRIGGER reports_consistency AFTER INSERT OR UPDATE OR DELETE ON app.reports
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION app.check_generation_consistency();

-- +goose StatementBegin
CREATE FUNCTION app.protect_generation_input() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND (NEW.user_id,NEW.collection_name,NEW.input_snapshot,NEW.scheduled_at,NEW.queued_at)
      IS DISTINCT FROM (OLD.user_id,OLD.collection_name,OLD.input_snapshot,OLD.scheduled_at,OLD.queued_at) THEN
        RAISE EXCEPTION 'Generation input is immutable' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.collection_id IS NOT NULL AND NEW.collection_id IS NULL AND OLD.status IN ('queued','running') THEN
        RAISE EXCEPTION 'Active Collection cannot be deleted' USING ERRCODE = '23514';
    END IF;
    IF NEW.collection_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM app.collections WHERE id=NEW.collection_id AND user_id=NEW.user_id) THEN
        RAISE EXCEPTION 'Collection owner mismatch' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER generation_input BEFORE INSERT OR UPDATE ON app.generation_jobs FOR EACH ROW EXECUTE FUNCTION app.protect_generation_input();
GRANT SELECT ON app.generation_jobs, app.generation_attempts, app.reports, app.report_target_results,
    app.report_facts, app.report_sources, app.fact_sources, app.report_source_failures TO watchbrief_api;
REVOKE ALL ON FUNCTION app.check_generation_consistency(), app.protect_generation_input() FROM PUBLIC;

-- +goose Down
DROP TABLE app.fact_sources, app.report_source_failures, app.report_facts, app.report_sources,
    app.report_target_results, app.reports, app.generation_attempts, app.generation_jobs;
DROP FUNCTION app.check_generation_consistency(), app.protect_generation_input();
