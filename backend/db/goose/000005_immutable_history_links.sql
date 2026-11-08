-- +goose Up
-- Parent changes would move ownership and bypass validation of the old successful parent.
-- Keep these identities and links immutable; only deleting completed settings may detach a Collection.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.protect_generation_input() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND (NEW.id,NEW.user_id,NEW.collection_name,NEW.input_snapshot,NEW.scheduled_at,NEW.queued_at)
      IS DISTINCT FROM (OLD.id,OLD.user_id,OLD.collection_name,OLD.input_snapshot,OLD.scheduled_at,OLD.queued_at) THEN
        RAISE EXCEPTION 'Generation input is immutable' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' AND NEW.collection_id IS DISTINCT FROM OLD.collection_id AND NEW.collection_id IS NOT NULL THEN
        RAISE EXCEPTION 'Generation Collection cannot be reassigned' USING ERRCODE = '23514';
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
-- +goose StatementBegin
CREATE FUNCTION app.protect_generation_links() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id THEN
        RAISE EXCEPTION 'History identity is immutable' USING ERRCODE = '23514';
    END IF;
    IF TG_TABLE_NAME = 'generation_attempts' THEN
        IF (NEW.generation_job_id,NEW.attempt_number) IS DISTINCT FROM (OLD.generation_job_id,OLD.attempt_number) THEN
            RAISE EXCEPTION 'Attempt parent is immutable' USING ERRCODE = '23514';
        END IF;
    ELSE
        IF NEW.generation_attempt_id IS DISTINCT FROM OLD.generation_attempt_id THEN
            RAISE EXCEPTION 'Report parent is immutable' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER generation_attempt_links BEFORE UPDATE ON app.generation_attempts
FOR EACH ROW EXECUTE FUNCTION app.protect_generation_links();
CREATE TRIGGER report_attempt_link BEFORE UPDATE ON app.reports
FOR EACH ROW EXECUTE FUNCTION app.protect_generation_links();
REVOKE ALL ON FUNCTION app.protect_generation_links() FROM PUBLIC;

-- +goose Down
DROP TRIGGER report_attempt_link ON app.reports;
DROP TRIGGER generation_attempt_links ON app.generation_attempts;
DROP FUNCTION app.protect_generation_links();
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.protect_generation_input() RETURNS trigger LANGUAGE plpgsql AS $$
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
