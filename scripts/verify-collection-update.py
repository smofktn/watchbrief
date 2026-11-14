#!/usr/bin/env python3
"""Body update checks, called only by the disposable verification runner."""
from concurrent.futures import ThreadPoolExecutor
import json
import uuid


def verify(request, sql, owner_token, other_token, seeded_id):
    original = dict(name='編集前の架空設定', research_instruction='作成入力の架空指示',
                    initial_lookback_mode='ai', initial_lookback_days=None)
    creation_key = str(uuid.uuid4())
    created = request('/api/collections', owner_token, original, extra_headers={'Idempotency-Key': creation_key})
    assert created[0] == 201
    collection_id = json.loads(created[2])['id']

    def detail(cid=collection_id):
        return json.loads(request('/api/collections/' + cid, owner_token)[2])

    def update(payload, key=None, token=owner_token, cid=collection_id, origin=None):
        options = dict(method='PATCH', extra_headers={'Idempotency-Key': key or str(uuid.uuid4())})
        if origin is not None:
            options['supplied_origin'] = origin
        return request('/api/collections/' + cid, token, payload, **options)

    def snap(cid=collection_id):
        return sql("SELECT row_to_json(c)::text FROM app.collections c WHERE id='" + cid + "'")

    first = {**original, 'name': '更新後の架空設定', 'research_instruction': '編集後の架空指示',
             'initial_lookback_mode': 'days', 'initial_lookback_days': 365, 'version': '1'}
    before = json.loads(snap())
    key = str(uuid.uuid4())
    saved = update(first, key)
    assert saved[0] == 200 and saved[1].get('Cache-Control') == 'no-store' and not saved[1].get('Set-Cookie')
    assert json.loads(saved[2]) == dict(id=collection_id, version='2')
    after = json.loads(snap())
    changed = {'name', 'research_instruction', 'initial_lookback_mode', 'initial_lookback_days', 'updated_at', 'version'}
    assert all(before[k] == after[k] for k in before if k not in changed)
    assert detail()['name'] == first['name'] and detail()['initial_lookback_days'] == 365
    assert any(c['id'] == collection_id and c['name'] == first['name'] for c in json.loads(request('/api/collections', owner_token)[2])['collections'])
    stable = snap()
    for _ in range(2):
        assert update(first, key)[0] == 200 and snap() == stable
    stable = snap()
    assert update({**first, 'name': '異なる入力'}, key)[0] == 409 and snap() == stable
    second = {**first, 'name': 'さらに更新', 'version': '2'}
    assert update(second)[0] == 200
    stable = snap()
    assert update(first, key)[0] == 200 and snap() == stable  # Replay never restores an old setting.
    replay = request('/api/collections', owner_token, original, extra_headers={'Idempotency-Key': creation_key})
    assert replay[0] == 200 and json.loads(replay[2])['id'] == collection_id and snap() == stable

    current = {**second, 'version': '3'}
    for bad in ({**current, 'name': '　\u0085'}, {**current, 'name': 'a' * 201},
                {**current, 'research_instruction': 'a' * 10001}, {**current, 'name': 'a\0'},
                {**current, 'initial_lookback_days': 0}, {**current, 'initial_lookback_days': 366},
                {**current, 'initial_lookback_mode': 'ai'}, {**current, 'version': '0'},
                {**current, 'version': 3}, {**current, 'enabled': True}, {**current, 'display_order': 9}):
        assert update(bad)[0] == 400 and snap() == stable
    assert update(current, token=None)[0] == 401
    for origin in ('', 'null', 'https://external.invalid'):
        assert update(current, origin=origin)[0] == 403
    hidden = update(current, token=other_token)
    missing = update(current, token=other_token, cid='00000000000000000000000000')
    assert hidden[0] == missing[0] == 404 and hidden[2] == missing[2]
    assert update(current, cid='invalid')[0] == 400
    assert update(first)[0] == 409 and snap() == stable
    # Equal request numbers belong to separate owners.
    other_id = json.loads(request('/api/collections', other_token)[2])['collections'][0]['id']
    other_before = json.loads(request('/api/collections/' + other_id, other_token)[2])
    other_input = {k: other_before[k] for k in original}
    independent = update({**other_input, 'version': other_before['version']}, key, token=other_token, cid=other_id)
    assert independent[0] == 200


    with ThreadPoolExecutor(max_workers=8) as executor:
        racing = list(executor.map(lambda i: update({**current, 'name': '並列変更' + str(i)}), range(8)))
    assert sum(r[0] == 200 for r in racing) == 1 and sum(r[0] == 409 for r in racing) == 7
    assert detail()['version'] == '4'
    concurrent_key = str(uuid.uuid4())
    same = {**current, 'version': '4'}
    with ThreadPoolExecutor(max_workers=8) as executor:
        repeats = list(executor.map(lambda _: update(same, concurrent_key), range(8)))
    assert all(r[0] == 200 and json.loads(r[2])['version'] == '5' for r in repeats)
    assert detail()['version'] == '5'
    # A key already used on another owned Collection is never a new operation.
    assert update(same, concurrent_key, cid=seeded_id)[0] == 409

    failing = {**current, 'version': '5'}
    failing_key = str(uuid.uuid4())
    stable = snap()
    sql("""CREATE FUNCTION app.verify_update_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fictional injected update failure'; END $$;
CREATE TRIGGER verify_update_failure BEFORE INSERT ON app.collection_update_requests
FOR EACH ROW EXECUTE FUNCTION app.verify_update_failure();""")
    try:
        assert update(failing, failing_key)[0] == 500 and snap() == stable
        assert sql("SELECT count(*) FROM app.collection_update_requests WHERE request_key='" + failing_key + "'") == '0'
    finally:
        sql('DROP TRIGGER verify_update_failure ON app.collection_update_requests; DROP FUNCTION app.verify_update_failure();')
    assert update(failing, failing_key)[0] == 200
    assert update(failing, failing_key)[0] == 200

    # Validate actual child settings and immutable completed/queued/running history.
    sql("""INSERT INTO app.generation_jobs(id,user_id,collection_id,collection_name,input_snapshot,scheduled_at,queued_at,status)
SELECT '00000000000000000000000011',user_id,collection_id,collection_name,input_snapshot,scheduled_at+interval '1 day',queued_at,'queued'
FROM app.generation_jobs WHERE collection_id='""" + seeded_id + "' LIMIT 1;")
    sql("""BEGIN;
INSERT INTO app.generation_jobs(id,user_id,collection_id,collection_name,input_snapshot,scheduled_at,queued_at,status)
SELECT '00000000000000000000000012',user_id,collection_id,collection_name,input_snapshot,scheduled_at+interval '2 days',queued_at,'running'
FROM app.generation_jobs WHERE collection_id='""" + seeded_id + """' LIMIT 1;
INSERT INTO app.generation_attempts(id,generation_job_id,attempt_number,started_at,heartbeat_at,status)
SELECT '00000000000000000000000013',id,1,queued_at,queued_at,'running' FROM app.generation_jobs WHERE id='00000000000000000000000012';
COMMIT;""")
    tables = ['collection_schedules', 'watch_targets', 'information_sources', 'generation_jobs', 'generation_attempts',
              'reports', 'report_target_results', 'report_facts', 'report_sources', 'fact_sources', 'report_source_failures',
              'collection_creation_requests']
    snapshots = {t: sql('SELECT json_agg(t ORDER BY to_jsonb(t)::text)::text FROM app.' + t + ' t') for t in tables}
    fixture = detail(seeded_id)
    fixture_input = {k: fixture[k] for k in original}
    assert update({**fixture_input, 'name': '履歴を変えない新しい名前', 'version': fixture['version']}, cid=seeded_id)[0] == 200
    assert all(sql('SELECT json_agg(t ORDER BY to_jsonb(t)::text)::text FROM app.' + t + ' t') == value for t, value in snapshots.items())
    assert update({**fixture_input, 'version': detail(seeded_id)['version']}, cid=seeded_id)[0] == 200
    # enabled is not editable, and an already enabled Collection is rejected.
    sql("UPDATE app.collections SET enabled=true WHERE id='" + collection_id + "'")
    assert update({**current, 'version': '6'})[0] == 409
    sql("UPDATE app.collections SET enabled=false,version=9223372036854775807 WHERE id='" + collection_id + "'")
    stable = snap()
    assert update({**current, 'version': '9223372036854775807'})[0] == 500 and snap() == stable
    # Delete only this disposable fixture; a replay must not resurrect it.
    sql("DELETE FROM app.collections WHERE id='" + collection_id + "'")
    assert update(first, key)[0] == 404
    assert request('/api/collections', owner_token, original, extra_headers={'Idempotency-Key': creation_key})[0] == 409
    for column in ('name', 'research_instruction', 'initial_lookback_mode', 'initial_lookback_days', 'version', 'updated_at'):
        assert sql("SELECT has_column_privilege('watchbrief_api','app.collections','" + column + "','UPDATE')") == 't'
    for statement in ['UPDATE app.collections SET enabled=true WHERE false', 'UPDATE app.collections SET display_order=1 WHERE false',
                      "UPDATE app.collections SET user_id='00000000000000000000000000' WHERE false", 'DELETE FROM app.collections WHERE false',
                      'UPDATE app.watch_targets SET enabled=true WHERE false', 'DELETE FROM app.collection_update_requests WHERE false',
                      'UPDATE app.collection_update_requests SET result_version=99 WHERE false',
                      'UPDATE app.collection_creation_requests SET input_digest=NULL WHERE false']:
        sql('BEGIN; SET LOCAL ROLE watchbrief_api; DO $verify$ BEGIN BEGIN ' + statement +
            "; RAISE EXCEPTION 'Unexpectedly allowed'; EXCEPTION WHEN insufficient_privilege THEN NULL; END; END $verify$; ROLLBACK;")
    print('Update API: owner/CSRF/input, version conflict, concurrent update/replay, atomic rollback/retry, creation replay, child/history invariance, overflow and least privileges: passed', flush=True)
