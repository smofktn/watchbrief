"""Exercise the home read contract using only fictional rows in the disposable DB."""
import json
import runpy
from datetime import date, datetime, time, timedelta, timezone


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
    assert selected_row == {'id': home_collection, 'name': '現在のCollection名（架空）', 'latest_report_id': good_ids['report'],
                            'today_job_counts': {'queued': 1, 'running': 1, 'succeeded': 2, 'failed': 1}}
    other_row = next(item for item in payload['collections'] if item['id'] == other_collection)
    assert other_row['latest_report_id'] == other_ids['report']
    assert other_row['today_job_counts'] == {'queued': 0, 'running': 0, 'succeeded': 1, 'failed': 0}
    assert all(set(item) == {'id', 'name', 'latest_report_id', 'today_job_counts'} for item in payload['collections'])
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
    assert empty_payload['collections'][-1] == {'id': empty_id, 'name': '空の架空Collection', 'latest_report_id': None,
                                                'today_job_counts': {'queued': 0, 'running': 0, 'succeeded': 0, 'failed': 0}}

    # Every GET above leaves the full saved-history tables byte-for-byte intact.
    assert snapshot() == fixture_history
    print('Home API: auth-first validation, owner scoping, shared JST date, four status counts, start/end microsecond boundaries, outside-day exclusion, latest Report despite newer non-success Jobs, updated counts after a new saved Job, current/empty Collections, and unchanged history: passed')
    return {'report_id': good_ids['report'], 'collection_id': home_collection, 'empty_collection_id': empty_id, 'as_of_date': as_of_date}


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
    print('Home API: deleted empty Collection disappears, zero-Collection response is empty, saved history remains accessible: passed')
