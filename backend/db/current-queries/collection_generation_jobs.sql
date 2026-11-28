-- name: ListOwnedCollectionGenerationJobs :many
WITH owned_collection AS MATERIALIZED (
    SELECT id FROM app.collections
    WHERE id = sqlc.arg(collection_id)::text AND user_id = sqlc.arg(user_id)::text
),
job_page AS MATERIALIZED (
    SELECT j.id, j.collection_name, j.scheduled_at, j.status, r.id AS report_id
    FROM app.generation_jobs j
    JOIN owned_collection c ON c.id = j.collection_id
    LEFT JOIN app.generation_attempts a ON a.generation_job_id = j.id
    LEFT JOIN app.reports r ON r.generation_attempt_id = a.id
    WHERE j.user_id = sqlc.arg(user_id)::text
    ORDER BY j.scheduled_at DESC, j.id DESC
    LIMIT 31
)
SELECT EXISTS (SELECT 1 FROM owned_collection) AS collection_exists,
       page.id, page.collection_name, page.scheduled_at, page.status, page.report_id
FROM (VALUES (1)) AS anchor(value)
LEFT JOIN job_page page ON TRUE
ORDER BY page.scheduled_at DESC NULLS LAST, page.id DESC NULLS LAST;

-- name: ListOwnedCollectionGenerationJobsBefore :many
WITH owned_collection AS MATERIALIZED (
    SELECT id FROM app.collections
    WHERE id = sqlc.arg(collection_id)::text AND user_id = sqlc.arg(user_id)::text
),
job_page AS MATERIALIZED (
    SELECT j.id, j.collection_name, j.scheduled_at, j.status, r.id AS report_id
    FROM app.generation_jobs j
    JOIN owned_collection c ON c.id = j.collection_id
    LEFT JOIN app.generation_attempts a ON a.generation_job_id = j.id
    LEFT JOIN app.reports r ON r.generation_attempt_id = a.id
    WHERE j.user_id = sqlc.arg(user_id)::text
      AND (j.scheduled_at, j.id) < (sqlc.arg(before_at)::timestamptz, sqlc.arg(before_id)::text)
    ORDER BY j.scheduled_at DESC, j.id DESC
    LIMIT 31
)
SELECT EXISTS (SELECT 1 FROM owned_collection) AS collection_exists,
       page.id, page.collection_name, page.scheduled_at, page.status, page.report_id
FROM (VALUES (1)) AS anchor(value)
LEFT JOIN job_page page ON TRUE
ORDER BY page.scheduled_at DESC NULLS LAST, page.id DESC NULLS LAST;
