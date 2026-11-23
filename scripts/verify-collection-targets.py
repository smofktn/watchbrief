#!/usr/bin/env python3
"""Atomic target edits, used exclusively by the disposable verification runner."""
from concurrent.futures import ThreadPoolExecutor
import json
import uuid


def verify(request, sql, owner_token, other_token, seeded_id):
    fields = ('name', 'research_instruction', 'initial_lookback_mode', 'initial_lookback_days', 'version')

    def detail(cid=seeded_id, token=owner_token):
        status, _, body = request('/api/collections/' + cid, token)
        assert status == 200
        return json.loads(body)

    def patch(changes, key=None, cid=seeded_id, token=owner_token, **options):
        return request('/api/collections/' + cid, token, changes, method='PATCH',
                       extra_headers={'Idempotency-Key': key or str(uuid.uuid4())}, **options)

    def snapshot():
        tables = ('collections', 'watch_targets', 'information_sources', 'collection_schedules',
                  'collection_update_requests', 'collection_creation_requests', 'generation_jobs',
                  'generation_attempts', 'reports', 'report_target_results', 'report_facts', 'report_sources',
                  'fact_sources', 'report_source_failures')
        parts = ["'" + t + "', (SELECT coalesce(json_agg(t ORDER BY to_jsonb(t)::text)::text,'[]') FROM app." + t + " t)" for t in tables]
        return json.loads(sql('SELECT json_build_object(' + ','.join(parts) + ')::text'))

    before = detail()
    existing = before['targets'][0]
    # Check a previously OFF target is not enabled by editing it.
    sql("UPDATE app.watch_targets SET enabled=false WHERE id='" + existing['id'] + "'")
    stable = snapshot()
    body = {k: before[k] for k in fields}
    body.update(name='本体と対象の一括保存', targets=[dict(id=existing['id'], name='変更した架空対象', instruction='改行\n 補足'),
                                                     dict(id=None, name='新しい架空対象', instruction='取得しない')])
    key = str(uuid.uuid4())
    result = patch(body, key)
    assert result[0] == 200
    after = detail()
    assert int(after['version']) == int(before['version']) + 1 and after['name'] == body['name'] and not after['enabled']
    old = next(t for t in after['targets'] if t['id'] == existing['id'])
    new = next(t for t in after['targets'] if t['name'] == '新しい架空対象')
    assert not old['enabled'] and old['sources'] == existing['sources'] and old['display_order'] == existing['display_order']
    assert new['enabled'] and new['sources'] == [] and new['display_order'] > max(t['display_order'] for t in before['targets'])
    snapshot_after = snapshot()
    assert all(snapshot_after[t] == stable[t] for t in stable if t not in ('collections', 'watch_targets', 'collection_update_requests'))
    with ThreadPoolExecutor(max_workers=8) as executor:
        replies = list(executor.map(lambda _: patch(body, key), range(8)))
    assert all(r[0] == 200 and r[2] == result[2] for r in replies) and snapshot() == snapshot_after
    assert patch({**body, 'name': '番号の使い回し'}, key)[0] == 409 and snapshot() == snapshot_after
    assert patch({k: before[k] for k in fields})[0] == 409  # stale body-only client
    # Replay old body-only requests after upgrade is covered by the original update suite.
    current = {k: after[k] for k in fields}
    current['targets'] = [dict(id=new['id'], name='再編集した架空対象', instruction='新しい補足')]
    assert patch(current)[0] == 200
    settled = snapshot()
    assert patch(body, key)[0] == 200 and snapshot() == settled  # old target request cannot undo edits
    # Cross-Collection IDs (same owner or different owner) and missing IDs return the same 404.
    other = json.loads(request('/api/collections', other_token)[2])['collections'][0]['id']
    foreign = detail(other, other_token)['targets'][0]['id']
    base = {k: detail()[k] for k in fields}
    hidden = patch({**base, 'targets': [dict(id=foreign, name='架空', instruction='')]})
    missing = patch({**base, 'targets': [dict(id='00000000000000000000000000', name='架空', instruction='')]})
    assert hidden[0] == missing[0] == 404 and hidden[2] == missing[2] and snapshot() == settled
    own_other = json.loads(request('/api/collections', owner_token)[2])['collections']
    cid = next(c['id'] for c in own_other if c['id'] != seeded_id)
    created = patch({**{k: detail(cid)[k] for k in fields}, 'targets': [dict(id=None, name='別のCollection対象', instruction='')]}, cid=cid)
    assert created[0] == 200
    wrong_id = detail(cid)['targets'][0]['id']
    settled = snapshot()
    assert patch({**base, 'targets': [dict(id=wrong_id, name='架空', instruction='')]})[0] == 404 and snapshot() == settled
    for changes in ([dict(id=None, name='　', instruction='')], [dict(id=None, name='x' * 201, instruction='')],
                    [dict(id=None, name='x', instruction='x' * 10001)], [dict(id=None, name='x\0', instruction='')],
                    [dict(id=existing['id'], name='x', instruction='')] * 2,
                    [dict(id=None, name='x', instruction='')] * 21):
        assert patch({**base, 'targets': changes})[0] == 400 and snapshot() == settled
    for supplied_origin in ('', 'null', 'https://external.invalid'):
        assert patch({**base, 'targets': [dict(id=None, name='x', instruction='')]}, supplied_origin=supplied_origin)[0] == 403
    assert patch({**base, 'targets': []}, token=None)[0] == 401
    # Maximum valid request crosses the old 64KiB boundary, without limiting total registered targets.
    large = {**base, 'targets': [dict(id=None, name='架空' + str(i), instruction='a' * 10000) for i in range(20)]}
    assert patch(large)[0] == 200 and len(detail()['targets']) == len(after['targets']) + 20
    # Different new requests racing the same version: one whole save only.
    base = {k: detail()[k] for k in fields}
    with ThreadPoolExecutor(max_workers=8) as executor:
        results = list(executor.map(lambda i: patch({**base, 'targets': [dict(id=None, name='並列対象' + str(i), instruction='')]}), range(8)))
    assert sum(r[0] == 200 for r in results) == 1 and sum(r[0] == 409 for r in results) == 7
    # Force failure after body and targets write: every write and request record must roll back.
    base = {k: detail()[k] for k in fields}
    failing = {**base, 'name': '巻き戻す本体', 'targets': [dict(id=existing['id'], name='巻き戻す既存対象', instruction=''), dict(id=None, name='巻き戻す追加', instruction='')]}
    failure_key = str(uuid.uuid4())
    settled = snapshot()
    sql("""CREATE FUNCTION app.verify_target_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fictional target failure'; END $$;
CREATE TRIGGER verify_target_failure BEFORE INSERT ON app.collection_update_requests
FOR EACH ROW EXECUTE FUNCTION app.verify_target_failure();""")
    try:
        assert patch(failing, failure_key)[0] == 500 and snapshot() == settled
    finally:
        sql('DROP TRIGGER verify_target_failure ON app.collection_update_requests; DROP FUNCTION app.verify_target_failure();')
    assert patch(failing, failure_key)[0] == 200
    settled = snapshot()
    assert patch(failing, failure_key)[0] == 200 and snapshot() == settled
    # Stopped parent permits editing with queued/running Jobs; immutable history already exists here.
    assert all(snapshot()[t] == stable[t] for t in stable if t in ('generation_jobs', 'generation_attempts', 'reports', 'report_target_results', 'report_facts', 'report_sources', 'fact_sources', 'report_source_failures'))
    # Verify actual minimum privileges, not only GRANT text.
    for statement in ('DELETE FROM app.watch_targets WHERE false', 'UPDATE app.watch_targets SET enabled=true WHERE false',
                      'UPDATE app.watch_targets SET display_order=3 WHERE false', 'UPDATE app.watch_targets SET collection_id=id WHERE false',
                      'UPDATE app.information_sources SET name=\'x\' WHERE false', 'UPDATE app.generation_jobs SET collection_name=\'x\' WHERE false'):
        sql('BEGIN; SET LOCAL ROLE watchbrief_api; DO $verify$ BEGIN BEGIN ' + statement +
            "; RAISE EXCEPTION 'Unexpectedly allowed'; EXCEPTION WHEN insufficient_privilege THEN NULL; END; END $verify$; ROLLBACK;")
    print('Target API: atomic body/add/edit, 8-way replay/race, old request replay, owner/CSRF/input/20 changes, >64KiB, rollback/retry, states/sources/history and minimum privileges: passed', flush=True)
