-- name: UserIDBySessionHash :one
SELECT user_id FROM app.auth_sessions WHERE token_hash = $1;

-- name: ListOwnedCollections :many
SELECT id, name, enabled, display_order FROM app.collections
WHERE user_id = $1 ORDER BY display_order, id;

-- name: OwnedCollectionDocument :one
SELECT jsonb_build_object(
    'id', c.id, 'name', c.name, 'enabled', c.enabled,
    'research_instruction', c.research_instruction,
    'initial_lookback_mode', c.initial_lookback_mode,
    'initial_lookback_days', c.initial_lookback_days,
    'display_order', c.display_order,
    'schedule', (SELECT jsonb_build_object('id', s.id, 'cron_expression', s.cron_expression)
        FROM app.collection_schedules s WHERE s.collection_id = c.id),
    'targets', COALESCE((SELECT jsonb_agg(jsonb_build_object(
        'id', t.id, 'name', t.name, 'instruction', t.instruction,
        'enabled', t.enabled, 'display_order', t.display_order,
        'sources', COALESCE((SELECT jsonb_agg(jsonb_build_object(
            'id', src.id, 'name', src.name, 'url', src.url, 'instruction', src.instruction
        ) ORDER BY src.id) FROM app.information_sources src WHERE src.watch_target_id = t.id), '[]'::jsonb)
    ) ORDER BY t.display_order, t.id) FROM app.watch_targets t WHERE t.collection_id = c.id), '[]'::jsonb)
) AS document FROM app.collections c WHERE c.id = $1 AND c.user_id = $2;
