#!/usr/bin/env python3
"""Creation checks called only inside verify-issue17's disposable project."""
from concurrent.futures import ThreadPoolExecutor
import json
import uuid


def verify(request, sql, owner_token, other_token):
    data = dict(name='新規保存の架空設定', research_instruction='再試行の整合性を確認する架空指示',
                initial_lookback_mode='ai', initial_lookback_days=None)
    def create(key, payload=data, token=owner_token, origin=None):
        options = dict(extra_headers={'Idempotency-Key': key})
        if origin is not None:
            options['supplied_origin'] = origin
        return request('/api/collections', token, payload, **options)

    key = str(uuid.uuid4())
    initial = create(key)
    assert initial[0] == 201 and initial[1].get('Cache-Control') == 'no-store'
    saved_id = json.loads(initial[2])['id']
    # Ignore the first response: the next request stands in for an application-level lost response.
    again = create(key)
    assert again[0] == 200 and json.loads(again[2])['id'] == saved_id
    detail = json.loads(request('/api/collections/' + saved_id, owner_token)[2])
    assert detail['name'] == data['name'] and detail['research_instruction'] == data['research_instruction']
    assert detail['enabled'] is False and detail['schedule'] is None and detail['targets'] == []
    assert detail['initial_lookback_mode'] == 'ai' and detail['initial_lookback_days'] is None
    assert create(key, {**data, 'name': '別の入力'})[0] == 409
    assert create(str(uuid.uuid4()), {**data, 'initial_lookback_mode': 'days', 'initial_lookback_days': 365})[0] == 201
    before = int(sql('SELECT count(*) FROM app.collections'))
    for payload in ({**data, 'name': '　 '}, {**data, 'initial_lookback_mode': 'days', 'initial_lookback_days': 0},
                    {**data, 'initial_lookback_days': 2}, {**data, 'enabled': True}, {**data, 'user_id': saved_id}):
        assert create(str(uuid.uuid4()), payload)[0] == 400
    assert request('/api/collections', None, data, extra_headers={'Idempotency-Key': str(uuid.uuid4())})[0] == 401
    for origin in ('null', 'https://external.invalid', ''):
        assert create(str(uuid.uuid4()), origin=origin)[0] == 403
    assert int(sql('SELECT count(*) FROM app.collections')) == before

    concurrent_key = str(uuid.uuid4())
    with ThreadPoolExecutor(max_workers=8) as executor:
        results = list(executor.map(lambda _: create(concurrent_key), range(8)))
    assert [r[0] for r in results].count(201) == 1
    assert all(r[0] in (200, 201) for r in results)
    assert len({json.loads(r[2])['id'] for r in results}) == 1
    with ThreadPoolExecutor(max_workers=8) as executor:
        distinct = list(executor.map(lambda _: create(str(uuid.uuid4())), range(8)))
    assert all(r[0] == 201 for r in distinct)
    ids = [json.loads(r[2])['id'] for r in distinct]
    orders = json.loads(sql("SELECT json_agg(display_order) FROM app.collections WHERE id IN (" + ','.join("'" + i + "'" for i in ids) + ")"))
    assert len(set(orders)) == len(ids)

    other = create(key, token=other_token)
    assert other[0] == 201 and json.loads(other[2])['id'] != saved_id
    hidden = request('/api/collections/' + saved_id, other_token)
    missing = request('/api/collections/00000000000000000000000000', other_token)
    assert hidden[0] == missing[0] == 404 and hidden[2] == missing[2]
    assert saved_id not in {r['id'] for r in json.loads(request('/api/collections', other_token)[2])['collections']}

    # Fail the second write after the Collection insert, then check both writes rolled back.
    failure_key = str(uuid.uuid4())
    sql("""CREATE FUNCTION app.verify_creation_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fictional injected failure'; END $$;
CREATE TRIGGER verify_creation_failure BEFORE INSERT ON app.collection_creation_requests
FOR EACH ROW EXECUTE FUNCTION app.verify_creation_failure();""")
    count = int(sql('SELECT count(*) FROM app.collections'))
    try:
        assert create(failure_key)[0] == 500
        assert int(sql('SELECT count(*) FROM app.collections')) == count
        assert sql("SELECT count(*) FROM app.collection_creation_requests WHERE request_key='" + failure_key + "'") == '0'
    finally:
        sql('DROP TRIGGER verify_creation_failure ON app.collection_creation_requests; DROP FUNCTION app.verify_creation_failure();')
    assert create(failure_key)[0] == 201
    assert create(failure_key)[0] == 200

    # Current settings may later be deleted; the original request must not resurrect them.
    sql("DELETE FROM app.collections WHERE id='" + saved_id + "'")
    assert create(key)[0] == 409
    assert sql("SELECT count(*) FROM app.collections WHERE id='" + saved_id + "'") == '0'

    # Verify least privileges explicitly, including an attempt to bypass the stopped default.
    checks = [
        ("SELECT has_column_privilege('watchbrief_api','app.collections','name','INSERT')", 't'),
        ("SELECT has_column_privilege('watchbrief_api','app.collections','enabled','INSERT')", 'f'),
        ("SELECT has_table_privilege('watchbrief_api','app.collections','UPDATE')", 'f'),
        ("SELECT has_table_privilege('watchbrief_api','app.collections','DELETE')", 'f'),
        ("SELECT has_table_privilege('watchbrief_api','app.collection_creation_requests','UPDATE')", 'f'),
        ("SELECT has_table_privilege('watchbrief_api','app.collection_creation_requests','DELETE')", 'f'),
        ("SELECT has_table_privilege('watchbrief_api','app.collection_schedules','INSERT')", 'f'),
        ("SELECT has_table_privilege('watchbrief_api','app.watch_targets','INSERT')", 'f'),
        ("SELECT has_table_privilege('watchbrief_user_creator','app.collection_creation_requests','SELECT')", 'f'),
    ]
    for statement, expected in checks:
        assert sql(statement) == expected
    for role, statement in [
        ('watchbrief_api', 'INSERT INTO app.collections(enabled) VALUES(true)'),
        ('watchbrief_api', 'UPDATE app.collections SET enabled=true WHERE false'),
        ('watchbrief_api', 'DELETE FROM app.collections WHERE false'),
        ('watchbrief_api', 'UPDATE app.collection_creation_requests SET input_digest=NULL WHERE false'),
        ('watchbrief_api', 'DELETE FROM app.collection_creation_requests WHERE false'),
        ('watchbrief_api', "INSERT INTO app.watch_targets(name) VALUES('fictional')"),
        ('watchbrief_user_creator', 'SELECT * FROM app.collection_creation_requests LIMIT 0'),
    ]:
        # A nested block accepts only an actual permission denial, never a different SQL error.
        sql("BEGIN; SET LOCAL ROLE " + role + "; DO $verify$ BEGIN BEGIN " + statement +
            "; RAISE EXCEPTION 'Unexpectedly allowed'; EXCEPTION WHEN insufficient_privilege THEN NULL; END; END $verify$; ROLLBACK;")
    print('Creation API: persistent replay, concurrent deduplication/order, input/CSRF/owner isolation, rollback/retry, deletion tombstone and minimal privileges: passed', flush=True)
