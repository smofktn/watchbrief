-- name: ListOwnedReports :many
SELECT r.id, j.collection_name, j.scheduled_at, a.finished_at AS generated_at
FROM app.reports r JOIN app.generation_attempts a ON a.id = r.generation_attempt_id
JOIN app.generation_jobs j ON j.id = a.generation_job_id
WHERE j.user_id = $1 ORDER BY j.scheduled_at DESC, r.id DESC;

-- name: OwnedReport :one
SELECT r.id, j.collection_name, j.scheduled_at, a.finished_at AS generated_at,
 r.body_markdown, r.summary_markdown, r.updates_started_at, a.started_at AS updates_ended_at
FROM app.reports r JOIN app.generation_attempts a ON a.id = r.generation_attempt_id
JOIN app.generation_jobs j ON j.id = a.generation_job_id
WHERE r.id = $1 AND j.user_id = $2;
