-- name: ListHomeCollections :many
SELECT c.id, c.name, COALESCE(latest.report_id, '') AS latest_report_id
FROM app.collections c
LEFT JOIN LATERAL (
    SELECT r.id AS report_id
    FROM app.generation_jobs j
    JOIN app.generation_attempts a ON a.generation_job_id = j.id
    JOIN app.reports r ON r.generation_attempt_id = a.id
    WHERE j.user_id = $1
      AND j.collection_id = c.id
      AND j.status = 'succeeded'
      AND a.status = 'succeeded'
    ORDER BY j.scheduled_at DESC, r.id DESC
    LIMIT 1
) latest ON TRUE
WHERE c.user_id = $1
ORDER BY c.display_order, c.id;
