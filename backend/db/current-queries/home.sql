-- name: ListHomeCollections :many
WITH today_jobs AS (
    SELECT j.collection_id,
           count(*) FILTER (WHERE j.status = 'queued') AS queued_count,
           count(*) FILTER (WHERE j.status = 'running') AS running_count,
           count(*) FILTER (WHERE j.status = 'succeeded') AS succeeded_count,
           count(*) FILTER (WHERE j.status = 'failed') AS failed_count
    FROM app.generation_jobs j
    WHERE j.user_id = sqlc.arg(user_id)
      AND j.collection_id IS NOT NULL
      AND j.scheduled_at >= sqlc.arg(start_at)::timestamptz
      AND j.scheduled_at < sqlc.arg(end_at)::timestamptz
    GROUP BY j.collection_id
)
SELECT c.id, c.name, c.version, COALESCE(latest.report_id, '') AS latest_report_id,
       COALESCE(today_jobs.queued_count, 0)::bigint AS today_queued_count,
       COALESCE(today_jobs.running_count, 0)::bigint AS today_running_count,
       COALESCE(today_jobs.succeeded_count, 0)::bigint AS today_succeeded_count,
       COALESCE(today_jobs.failed_count, 0)::bigint AS today_failed_count
FROM app.collections c
LEFT JOIN today_jobs ON today_jobs.collection_id = c.id
LEFT JOIN LATERAL (
    SELECT r.id AS report_id
    FROM app.generation_jobs j
    JOIN app.generation_attempts a ON a.generation_job_id = j.id
    JOIN app.reports r ON r.generation_attempt_id = a.id
    WHERE j.user_id = sqlc.arg(user_id)
      AND j.collection_id = c.id
      AND j.status = 'succeeded'
      AND a.status = 'succeeded'
    ORDER BY j.scheduled_at DESC, r.id DESC
    LIMIT 1
) latest ON TRUE
WHERE c.user_id = sqlc.arg(user_id)
ORDER BY c.display_order, c.id;
