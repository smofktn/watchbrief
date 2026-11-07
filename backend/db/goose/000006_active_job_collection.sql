-- +goose Up
-- NULL represents retained history after deleting completed settings, never an active Job.
ALTER TABLE app.generation_jobs ADD CONSTRAINT active_job_requires_collection
CHECK (collection_id IS NOT NULL OR status IN ('succeeded', 'failed'));

-- +goose Down
ALTER TABLE app.generation_jobs DROP CONSTRAINT active_job_requires_collection;
