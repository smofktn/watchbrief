"""Exercise the home read contract using only fictional rows in the disposable DB."""
import json
import runpy
from concurrent.futures import ThreadPoolExecutor
from datetime import date, datetime, time, timedelta, timezone


def verify(request, sql, token, other_token, order_one_token, login_id):
    owner = sql("SELECT id FROM app.users WHERE login_id='" + login_id + "'")
    report_seed = runpy.run_path('scripts/seed-reports.py')
    ids, _ = report_seed['fixture_rows'](owner)
    home_collection = ids['collection']
    tables = ('generation_jobs', 'generation_attempts', 'reports', 'report_target_results',
              'report_facts', 'report_sources', 'fact_sources', 'report_source_failures')

    def snapshot():
        return {table: sql("SELECT coalesce(json_agg(t ORDER BY id)::text,'[]') FROM app." + table + ' t')
                for table in tables}

    before = snapshot()
    assert request('/api/home', None)[0] == 401
    initial_status, _, initial_body = request('/api/home', token)
    assert initial_status == 200
    as_of_date = json.loads(initial_body)['as_of_date']
    day = date.fromisoformat(as_of_date)
    jst = timezone(timedelta(hours=9))
    start = datetime.combine(day, time.min, tzinfo=jst).astimezone(timezone.utc)
    end = start + timedelta(days=1)

    def scheduled_at(offset_microseconds):
        return (start + timedelta(microseconds=offset_microseconds)).isoformat(timespec='microseconds').replace('+00:00', 'Z')

    def day_end_minus_one_microsecond():
        return (end - timedelta(microseconds=1)).isoformat(timespec='microseconds').replace('+00:00', 'Z')

    status, _, body = request('/api/home?unexpected=1', token)
    assert status == 400
    assert request('/api/home?unexpected=1', None)[0] == 401

    # Insert a real newer success Report. Then put valid queued/running/failed
    # jobs after it; none may hide the most recent success.
    good_ids, good_rows = report_seed['fixture_rows'](owner + ':home-good')
    collection_fixture = runpy.run_path('scripts/seed-collections.py')
    other_collection = collection_fixture['fixture_id'](owner, 'collection')
    other_ids, other_rows = report_seed['fixture_rows'](owner + ':home-other-collection')
    selected = []
    for fixture_ids, rows, collection_id, historical_name in (
        (good_ids, good_rows, home_collection, '成功時の履歴名（架空）'),
        (other_ids, other_rows, other_collection, '別Collection同時刻の履歴名（架空）'),
    ):
        for table, original in rows:
            if table == 'collections':
                continue
            row = dict(original)
            if table == 'generation_jobs':
                row.update(user_id=owner, collection_id=collection_id,
                           collection_name=historical_name, scheduled_at=scheduled_at(500000))
                snapshot_data = json.loads(row['input_snapshot'])
                snapshot_data['collection_id'] = collection_id
                row['input_snapshot'] = json.dumps(snapshot_data, ensure_ascii=False)
            if table == 'watch_targets':
                row['collection_id'] = collection_id
            selected.append((table, row))

    daily = runpy.run_path('scripts/seed-daily-jobs.py')
    _, fixtures = daily['fixture_rows'](owner, 6, home_collection, '当日状態の履歴名（架空）')
    job_indexes = {}
    current_index = -1
    for table, row in fixtures:
        if table == 'collections':
            current_index += 1
            continue
        if table == 'generation_jobs':
            boundary_times = [scheduled_at(0), scheduled_at(600000), scheduled_at(700000), day_end_minus_one_microsecond(), end.isoformat(timespec='microseconds').replace('+00:00', 'Z'), (start - timedelta(microseconds=1)).isoformat(timespec='microseconds').replace('+00:00', 'Z')]
            row.update(scheduled_at=boundary_times[current_index], collection_name='当日状態の履歴名（架空）')
            if current_index == 4:
                row['status'] = 'queued'
            row['collection_id'] = home_collection
            job_indexes[row['id']] = current_index
            selected.append((table, row))
        elif table == 'generation_attempts':
            # The successful item includes its matching Report; the remaining
            # active/failed states retain their corresponding valid shapes.
            index = job_indexes[row['generation_job_id']]
            if index != 4:
                selected.append((table, row))
        elif current_index == 0:
            if table == 'watch_targets':
                row['collection_id'] = home_collection
            selected.append((table, row))

    # Use the seed helper's SQL literal function and insert immutable synthetic rows.
    literal = report_seed['literal']
    sql('BEGIN; SET LOCAL ROLE watchbrief_migrator;' + ''.join(
        'INSERT INTO app.' + table + '(' + ','.join(row) + ') VALUES (' + ','.join(literal(value) for value in row.values()) + ');'
        for table, row in selected) + 'COMMIT;')
    sql("UPDATE app.collections SET name='現在のCollection名（架空）' WHERE id='" + home_collection + "'")
    fixture_history = snapshot()

    status, _, body = request('/api/home', token)
    assert status == 200
    payload = json.loads(body)
    assert set(payload) == {'as_of_date', 'collections'}
    assert payload['as_of_date'] == as_of_date
    selected_row = next(item for item in payload['collections'] if item['id'] == home_collection)
    home_version = sql("SELECT version FROM app.collections WHERE id='" + home_collection + "'")
    other_version = sql("SELECT version FROM app.collections WHERE id='" + other_collection + "'")
    assert selected_row == {'id': home_collection, 'name': '現在のCollection名（架空）', 'version': home_version, 'latest_report_id': good_ids['report'],
                            'today_job_counts': {'queued': 1, 'running': 1, 'succeeded': 2, 'failed': 1}}
    other_row = next(item for item in payload['collections'] if item['id'] == other_collection)
    assert other_row['latest_report_id'] == other_ids['report']
    assert other_row['today_job_counts'] == {'queued': 0, 'running': 0, 'succeeded': 1, 'failed': 0}
    assert all(set(item) == {'id', 'name', 'version', 'latest_report_id', 'today_job_counts'} for item in payload['collections'])
    assert other_row['version'] == other_version
    assert request('/api/reports/' + good_ids['report'], token)[0] == 200
    assert snapshot() == fixture_history

    # Add a new fictional queued Job after the first response. A subsequent home
    # read, as the manual refresh does, must reflect the changed saved state.
    refresh_seed = runpy.run_path('scripts/seed-daily-jobs.py')
    _, refresh_rows = refresh_seed['fixture_rows'](owner + ':home-refresh', 2, home_collection, '更新確認の履歴名（架空）')
    refresh_job = next(row for table, row in refresh_rows if table == 'generation_jobs' and row['status'] == 'queued')
    refresh_job['user_id'] = owner
    refresh_job['collection_id'] = home_collection
    refresh_job['scheduled_at'] = scheduled_at(800000)
    refresh_job['collection_name'] = '更新確認の履歴名（架空）'
    refresh_snapshot = json.loads(refresh_job['input_snapshot'])
    refresh_snapshot['collection_id'] = home_collection
    refresh_job['input_snapshot'] = json.dumps(refresh_snapshot, ensure_ascii=False)
    sql('BEGIN; SET LOCAL ROLE watchbrief_migrator; INSERT INTO app.generation_jobs(' + ','.join(refresh_job) + ') VALUES (' + ','.join(literal(value) for value in refresh_job.values()) + '); COMMIT;')
    fixture_history = snapshot()
    refreshed_status, _, refreshed_body = request('/api/home', token)
    assert refreshed_status == 200
    refreshed_row = next(item for item in json.loads(refreshed_body)['collections'] if item['id'] == home_collection)
    assert refreshed_row['today_job_counts'] == {'queued': 2, 'running': 1, 'succeeded': 2, 'failed': 1}
    assert refreshed_row['latest_report_id'] == good_ids['report']
    assert request('/api/home', other_token)[0] == 200
    other_payload = json.loads(request('/api/home', other_token)[2])
    assert all(item['id'] != home_collection for item in other_payload['collections'])

    # A second current Collection without history remains visible with a null link.
    empty_id = '00000000000000000000000091'
    sql("INSERT INTO app.collections(id,user_id,name,research_instruction,enabled,initial_lookback_mode,display_order) VALUES ('" +
        empty_id + "','" + owner + "','空の架空Collection','検証用',false,'ai',99)")
    empty_payload = json.loads(request('/api/home', token)[2])
    empty_version = sql("SELECT version FROM app.collections WHERE id='" + empty_id + "'")
    assert empty_payload['collections'][-1] == {'id': empty_id, 'name': '空の架空Collection', 'version': empty_version, 'latest_report_id': None,
                                                'today_job_counts': {'queued': 0, 'running': 0, 'succeeded': 0, 'failed': 0}}

    # Exercise the minimal full-list reorder through the authenticated API.
    before_order = json.loads(request('/api/home', token)[2])['collections']
    before_ids = [item['id'] for item in before_order]
    original_positions = {row['id']: sql("SELECT display_order FROM app.collections WHERE id='" + row['id'] + "'") for row in before_order}
    original_versions = {row['id']: row['version'] for row in before_order}
    wanted = list(reversed(before_ids))
    reorder_body = {'expected': [{'id': row['id'], 'version': row['version']} for row in before_order], 'ordered_ids': wanted}
    reorder_key = '00112233-4455-4677-8899-aabbccddeeff'
    reorder_headers = {'Idempotency-Key': reorder_key}
    assert request('/api/collections/order?unexpected=1', None, raw_body=b'{', extra_headers=reorder_headers, method='PUT')[0] == 401
    assert request('/api/collections/order', None, reorder_body, extra_headers=reorder_headers, method='PUT')[0] == 401
    assert request('/api/collections/order', token, reorder_body, supplied_origin=None, extra_headers=reorder_headers, method='PUT')[0] == 403
    first_expected = reorder_body['expected'][0]
    duplicate_rows = (
        '{"id":' + json.dumps(first_expected['id']) + ',"id":' + json.dumps(first_expected['id']) + ',"version":' + json.dumps(first_expected['version']) + '}',
        '{"id":' + json.dumps(first_expected['id']) + ',"id":' + json.dumps(before_ids[1]) + ',"version":' + json.dumps(first_expected['version']) + '}',
        '{"id":' + json.dumps(first_expected['id']) + ',"version":' + json.dumps(first_expected['version']) + ',"version":' + json.dumps(first_expected['version']) + '}',
        '{"id":' + json.dumps(first_expected['id']) + ',"version":' + json.dumps(first_expected['version']) + ',"version":"2"}',
    )
    for duplicate_row in duplicate_rows:
        raw = ('{"expected":[' + duplicate_row + '],"ordered_ids":[' + json.dumps(first_expected['id']) + ']}').encode()
        invalid_status, _, invalid_body = request('/api/collections/order', token, extra_headers=reorder_headers, method='PUT', raw_body=raw)
        assert invalid_status == 400 and json.loads(invalid_body)['error']['code'] == 'invalid_request'
    duplicate_ids = {'expected': reorder_body['expected'], 'ordered_ids': [before_ids[0], before_ids[0], *before_ids[2:]]}
    assert request('/api/collections/order', token, duplicate_ids, extra_headers=reorder_headers, method='PUT')[0] == 400
    missing_id = {'expected': reorder_body['expected'], 'ordered_ids': before_ids[:-1]}
    assert request('/api/collections/order', token, missing_id, extra_headers=reorder_headers, method='PUT')[0] == 400
    foreign_id = sql("SELECT id FROM app.collections WHERE user_id=(SELECT id FROM app.users WHERE login_id='verify_e2e_other') ORDER BY id LIMIT 1")
    foreign_expected = [dict(row) for row in reorder_body['expected']]
    foreign_expected[0]['id'] = foreign_id
    foreign_ordered = list(wanted)
    foreign_ordered[foreign_ordered.index(before_ids[0])] = foreign_id
    foreign_body = {'expected': foreign_expected, 'ordered_ids': foreign_ordered}
    other_collection_rows = sql("SELECT coalesce(json_agg(c ORDER BY id)::text,'[]') FROM app.collections c WHERE user_id=(SELECT id FROM app.users WHERE login_id='verify_e2e_other')")
    foreign_status, _, foreign_response = request('/api/collections/order', token, foreign_body, extra_headers=reorder_headers, method='PUT')
    assert foreign_status == 409 and json.loads(foreign_response)['error']['code'] == 'order_conflict'
    assert sql("SELECT coalesce(json_agg(c ORDER BY id)::text,'[]') FROM app.collections c WHERE user_id=(SELECT id FROM app.users WHERE login_id='verify_e2e_other')") == other_collection_rows
    noop_key = '30112233-4455-4677-8899-aabbccddeeff'
    noop_body = {'expected': reorder_body['expected'], 'ordered_ids': before_ids}
    with ThreadPoolExecutor(max_workers=8) as executor:
        repeated_noops = list(executor.map(lambda _: request('/api/collections/order', token, noop_body,
            extra_headers={'Idempotency-Key': noop_key}, method='PUT')[0], range(8)))
    assert repeated_noops == [204] * 8
    distinct_keys = ['40112233-4455-4677-8899-aabbccddeef' + format(index, 'x') for index in range(8)]
    with ThreadPoolExecutor(max_workers=8) as executor:
        competing = list(executor.map(lambda key: request('/api/collections/order', token, reorder_body,
            extra_headers={'Idempotency-Key': key}, method='PUT')[0], distinct_keys))
    assert competing.count(204) == 1 and competing.count(409) == 7
    saved_status, _, saved_body = request('/api/collections/order', token, reorder_body, extra_headers=reorder_headers, method='PUT')
    assert saved_status == 409 and json.loads(saved_body)['error']['code'] == 'order_conflict'
    reordered = json.loads(request('/api/home', token)[2])['collections']
    assert [row['id'] for row in reordered] == wanted
    for index, row in enumerate(reordered):
        assert sql("SELECT display_order FROM app.collections WHERE id='" + row['id'] + "'") == str(index)
        expected_version = int(original_versions[row['id']]) + (original_positions[row['id']] != str(index))
        assert row['version'] == str(expected_version)
    before_noop_replay = sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'")
    assert request('/api/collections/order', token, noop_body, extra_headers={'Idempotency-Key': noop_key}, method='PUT')[0] == 204
    assert sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'") == before_noop_replay
    different_body = dict(noop_body, ordered_ids=wanted)
    conflict_status, _, conflict_body = request('/api/collections/order', token, different_body, extra_headers={'Idempotency-Key': noop_key}, method='PUT')
    assert conflict_status == 409 and json.loads(conflict_body)['error']['code'] == 'order_request_conflict'
    rollback_current = json.loads(request('/api/home', token)[2])['collections']
    rollback_body = {'expected': [{'id': row['id'], 'version': row['version']} for row in rollback_current],
                     'ordered_ids': list(reversed([row['id'] for row in rollback_current]))}
    rollback_key = '50112233-4455-4677-8899-aabbccddeeff'
    order_state = sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'")
    sql("""CREATE FUNCTION app.verify_order_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fictional injected order failure'; END $$;
CREATE TRIGGER verify_order_failure BEFORE INSERT ON app.collection_order_requests
FOR EACH ROW EXECUTE FUNCTION app.verify_order_failure();""")
    try:
        failure_status, _, failure_body = request('/api/collections/order', token, rollback_body,
            extra_headers={'Idempotency-Key': rollback_key}, method='PUT')
        assert failure_status == 500 and json.loads(failure_body)['error']['code'] == 'internal_error'
        assert sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'") == order_state
        assert sql("SELECT count(*) FROM app.collection_order_requests WHERE user_id='" + owner + "' AND request_key='" + rollback_key + "'") == '0'
    finally:
        sql('DROP TRIGGER verify_order_failure ON app.collection_order_requests; DROP FUNCTION app.verify_order_failure();')
    assert request('/api/collections/order', token, rollback_body, extra_headers={'Idempotency-Key': rollback_key}, method='PUT')[0] == 204
    committed_order = sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'")
    assert committed_order != order_state
    assert request('/api/collections/order', token, rollback_body, extra_headers={'Idempotency-Key': rollback_key}, method='PUT')[0] == 204
    assert sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'") == committed_order
    # Shared namespace-18 owner locks serialize concurrent create/order and
    # edit/order requests. The loser must see the changed complete snapshot.
    create_race_rows = json.loads(request('/api/home', token)[2])['collections']
    create_race_order = {'expected': [{'id': row['id'], 'version': row['version']} for row in create_race_rows],
                         'ordered_ids': list(reversed([row['id'] for row in create_race_rows]))}
    with ThreadPoolExecutor(max_workers=2) as executor:
        order_future = executor.submit(request, '/api/collections/order', token, create_race_order,
            extra_headers={'Idempotency-Key': '90112233-4455-4677-8899-aabbccddeeff'}, method='PUT')
        create_future = executor.submit(request, '/api/collections', token,
            {'name': '並行作成の架空Collection', 'research_instruction': '並行処理用', 'initial_lookback_mode': 'ai', 'initial_lookback_days': None},
            extra_headers={'Idempotency-Key': '91112233-4455-4677-8899-aabbccddeeff'})
        order_result, create_result = order_future.result(), create_future.result()
    assert order_result[0] in (204, 409) and create_result[0] == 201
    assert order_result[0] == 204 or json.loads(order_result[2])['error']['code'] == 'order_conflict'
    assert any(row['name'] == '並行作成の架空Collection' for row in json.loads(request('/api/home', token)[2])['collections'])
    edit_race_rows = json.loads(request('/api/home', token)[2])['collections']
    edit_detail = json.loads(request('/api/collections/' + home_collection, token)[2])
    edit_race_order = {'expected': [{'id': row['id'], 'version': row['version']} for row in edit_race_rows],
                       'ordered_ids': list(reversed([row['id'] for row in edit_race_rows]))}
    edit_body = {key: edit_detail[key] for key in ('name', 'research_instruction', 'initial_lookback_mode', 'initial_lookback_days')}
    edit_body['research_instruction'] += ' 並行編集確認'
    edit_body['version'] = edit_detail['version']
    with ThreadPoolExecutor(max_workers=2) as executor:
        order_future = executor.submit(request, '/api/collections/order', token, edit_race_order,
            extra_headers={'Idempotency-Key': '92112233-4455-4677-8899-aabbccddeeff'}, method='PUT')
        edit_future = executor.submit(request, '/api/collections/' + home_collection, token, edit_body,
            extra_headers={'Idempotency-Key': '93112233-4455-4677-8899-aabbccddeeff'}, method='PATCH')
        order_result, edit_result = order_future.result(), edit_future.result()
    assert (order_result[0], edit_result[0]) in ((204, 409), (409, 200))
    if order_result[0] == 409:
        assert json.loads(order_result[2])['error']['code'] == 'order_conflict'
        assert edit_result[0] == 200
        assert json.loads(edit_result[2])['version'] != edit_detail['version']
        assert json.loads(request('/api/collections/' + home_collection, token)[2])['research_instruction'] == edit_body['research_instruction']
    else:
        assert edit_result[0] == 409 and json.loads(edit_result[2])['error']['code'] == 'update_conflict'
    stale_key = '10112233-4455-4677-8899-aabbccddeeff'
    stale_status, _, stale_body = request('/api/collections/order', token, reorder_body, extra_headers={'Idempotency-Key': stale_key}, method='PUT')
    assert stale_status == 409 and json.loads(stale_body)['error']['code'] == 'order_conflict'
    # No-op requests with sparse/tied stored positions do not normalize or bump versions.
    sql("UPDATE app.collections SET display_order=7 WHERE id IN ('" + home_collection + "','" + other_collection + "')")
    one = json.loads(request('/api/home', token)[2])['collections']
    one_expected = [{'id': row['id'], 'version': row['version']} for row in one]
    one_key = '20112233-4455-4677-8899-aabbccddeeff'
    one_body = {'expected': one_expected, 'ordered_ids': [row['id'] for row in one]}
    one_before = sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'")
    assert request('/api/collections/order', token, one_body, extra_headers={'Idempotency-Key': one_key}, method='PUT')[0] == 204
    assert sql("SELECT coalesce(json_agg(json_build_array(id,display_order,version,updated_at::text) ORDER BY id)::text,'[]') FROM app.collections WHERE user_id='" + owner + "'") == one_before
    one_owner = sql("SELECT id FROM app.users WHERE login_id='verify_e2e_order_one'")
    one_other_id = sql("SELECT id FROM app.collections WHERE user_id='" + one_owner + "' ORDER BY display_order,id LIMIT 1")
    sql("UPDATE app.collections SET display_order=7 WHERE id='" + one_other_id + "'")
    other_one = json.loads(request('/api/home', order_one_token)[2])['collections']
    assert len(other_one) == 1 and other_one[0]['id'] == one_other_id
    other_one_state = sql("SELECT display_order||':'||version||':'||updated_at::text FROM app.collections WHERE id='" + one_other_id + "'")
    assert request('/api/collections/order', order_one_token,
        {'expected': [{'id': other_one[0]['id'], 'version': other_one[0]['version']}], 'ordered_ids': [one_other_id]},
        extra_headers={'Idempotency-Key': '60112233-4455-4677-8899-aabbccddeeff'}, method='PUT')[0] == 204
    assert sql("SELECT display_order||':'||version||':'||updated_at::text FROM app.collections WHERE id='" + one_other_id + "'") == other_one_state

    # Every GET above leaves the full saved-history tables byte-for-byte intact.
    assert snapshot() == fixture_history
    print('Home and order APIs: auth-first validation, owner/CSRF isolation, malformed input, concurrent replay/conflict, sparse/tied no-op, rollback/retry, shared JST date/counts, latest Report, Collection/version order persistence, and unchanged history: passed')
    final_home = json.loads(request('/api/home', token)[2])['collections']
    return {'report_id': good_ids['report'], 'collection_id': home_collection, 'empty_collection_id': empty_id, 'as_of_date': as_of_date,
            'order_names': [row['name'] for row in final_home], 'order_ids': [row['id'] for row in final_home]}


def verify_deleted_and_empty(request, sql, token, other_token, other_login_id, other_report_id, checks):
    sql("DELETE FROM app.collections WHERE id='" + checks['empty_collection_id'] + "'")
    assert all(item['id'] != checks['empty_collection_id'] for item in json.loads(request('/api/home', token)[2])['collections'])
    other_owner = sql("SELECT id FROM app.users WHERE login_id='" + other_login_id + "'")
    sql("DELETE FROM app.collections WHERE user_id='" + other_owner + "'")
    status, _, body = request('/api/home', other_token)
    empty_payload = json.loads(body)
    assert status == 200 and empty_payload['collections'] == [] and len(empty_payload['as_of_date']) == 10
    assert sql("SELECT count(*) FROM app.generation_jobs WHERE user_id='" + other_owner + "'") == '1'
    assert sql("SELECT count(*) FROM app.reports r JOIN app.generation_attempts a ON a.id=r.generation_attempt_id JOIN app.generation_jobs j ON j.id=a.generation_job_id WHERE j.user_id='" + other_owner + "'") == '1'
    assert request('/api/reports/' + other_report_id, other_token)[0] == 200
    assert request('/api/generation-jobs', other_token)[0] == 200
    zero_body = {'expected': [], 'ordered_ids': []}
    zero_headers = {'Idempotency-Key': '70112233-4455-4677-8899-aabbccddeeff'}
    assert request('/api/collections/order', other_token, zero_body, extra_headers=zero_headers, method='PUT')[0] == 204
    assert request('/api/collections/order', other_token, zero_body, extra_headers=zero_headers, method='PUT')[0] == 204
    print('Home API: deleted empty Collection disappears, zero-Collection response is empty, saved history remains accessible: passed')
