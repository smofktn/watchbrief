"""Exercise the home read contract using only fictional rows in the disposable DB."""
import json
import runpy


def verify(request, sql, token, other_token, login_id):
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
    assert request('/api/home', token)[0] == 200
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
                           collection_name=historical_name, scheduled_at='2026-11-10T00:00:00Z')
                snapshot_data = json.loads(row['input_snapshot'])
                snapshot_data['collection_id'] = collection_id
                row['input_snapshot'] = json.dumps(snapshot_data, ensure_ascii=False)
            if table == 'watch_targets':
                row['collection_id'] = collection_id
            selected.append((table, row))

    daily = runpy.run_path('scripts/seed-daily-jobs.py')
    _, fixtures = daily['fixture_rows'](owner, 4, home_collection, '非成功の履歴名（架空）')
    job_indexes = {}
    current_index = -1
    for table, row in fixtures:
        if table == 'collections':
            current_index += 1
            continue
        if table == 'generation_jobs':
            day = 1 if current_index == 0 else 10 + current_index
            row.update(scheduled_at=f'2026-11-{day:02d}T00:00:00Z', collection_name='非成功の履歴名（架空）')
            row['collection_id'] = home_collection
            job_indexes[row['id']] = current_index
            selected.append((table, row))
        elif table == 'generation_attempts':
            # The successful item includes its matching Report; the remaining
            # active/failed states retain their corresponding valid shapes.
            index = job_indexes[row['generation_job_id']]
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
    assert set(payload) == {'collections'}
    selected_row = next(item for item in payload['collections'] if item['id'] == home_collection)
    assert selected_row == {'id': home_collection, 'name': '現在のCollection名（架空）', 'latest_report_id': good_ids['report']}
    other_row = next(item for item in payload['collections'] if item['id'] == other_collection)
    assert other_row['latest_report_id'] == other_ids['report']
    assert all(set(item) == {'id', 'name', 'latest_report_id'} for item in payload['collections'])
    assert request('/api/reports/' + good_ids['report'], token)[0] == 200
    assert request('/api/home', other_token)[0] == 200
    other_payload = json.loads(request('/api/home', other_token)[2])
    assert all(item['id'] != home_collection for item in other_payload['collections'])

    # A second current Collection without history remains visible with a null link.
    empty_id = '00000000000000000000000091'
    sql("INSERT INTO app.collections(id,user_id,name,research_instruction,enabled,initial_lookback_mode,display_order) VALUES ('" +
        empty_id + "','" + owner + "','空の架空Collection','検証用',false,'ai',99)")
    empty_payload = json.loads(request('/api/home', token)[2])
    assert empty_payload['collections'][-1] == {'id': empty_id, 'name': '空の架空Collection', 'latest_report_id': None}

    # Every GET above leaves the full saved-history tables byte-for-byte intact.
    assert snapshot() == fixture_history
    print('Home API: auth-first query validation, owner scoping, multiple successes, same-time cross-Collection isolation, matching Job+Attempt success, newer queued/running/failed filtering, current and empty Collections, and unchanged saved history: passed')
    return {'report_id': good_ids['report'], 'collection_id': home_collection, 'empty_collection_id': empty_id}


def verify_deleted_and_empty(request, sql, token, other_token, other_login_id, other_report_id, checks):
    sql("DELETE FROM app.collections WHERE id='" + checks['empty_collection_id'] + "'")
    assert all(item['id'] != checks['empty_collection_id'] for item in json.loads(request('/api/home', token)[2])['collections'])
    other_owner = sql("SELECT id FROM app.users WHERE login_id='" + other_login_id + "'")
    sql("DELETE FROM app.collections WHERE user_id='" + other_owner + "'")
    status, _, body = request('/api/home', other_token)
    assert status == 200 and json.loads(body) == {'collections': []}
    assert sql("SELECT count(*) FROM app.generation_jobs WHERE user_id='" + other_owner + "'") == '1'
    assert sql("SELECT count(*) FROM app.reports r JOIN app.generation_attempts a ON a.id=r.generation_attempt_id JOIN app.generation_jobs j ON j.id=a.generation_job_id WHERE j.user_id='" + other_owner + "'") == '1'
    assert request('/api/reports/' + other_report_id, other_token)[0] == 200
    assert request('/api/generation-jobs', other_token)[0] == 200
    print('Home API: deleted empty Collection disappears, zero-Collection response is empty, saved history remains accessible: passed')
