-- name: ListOwnedGenerationJobs :many
SELECT j.id,j.collection_name,j.scheduled_at,j.status,r.id AS report_id
FROM app.generation_jobs j
LEFT JOIN app.generation_attempts a ON a.generation_job_id=j.id
LEFT JOIN app.reports r ON r.generation_attempt_id=a.id
WHERE j.user_id=$1
ORDER BY j.scheduled_at DESC,j.id DESC LIMIT 31;

-- name: ListOwnedGenerationJobsBefore :many
SELECT j.id,j.collection_name,j.scheduled_at,j.status,r.id AS report_id
FROM app.generation_jobs j
LEFT JOIN app.generation_attempts a ON a.generation_job_id=j.id
LEFT JOIN app.reports r ON r.generation_attempt_id=a.id
WHERE j.user_id=sqlc.arg(user_id) AND (j.scheduled_at,j.id)<(sqlc.arg(before_at)::timestamptz,sqlc.arg(before_id)::text)
ORDER BY j.scheduled_at DESC,j.id DESC LIMIT 31;
