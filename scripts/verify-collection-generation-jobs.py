#!/usr/bin/env python3
"""API boundary and ownership checks against the disposable verification DB."""
import base64
import hashlib
import json
import runpy
import secrets

seed = runpy.run_path(__file__.replace('verify-collection-generation-jobs.py', 'seed-daily-jobs.py'))
literal = seed['original']['literal']
TABLES = ('generation_jobs','generation_attempts','reports','report_target_results','report_facts','report_sources','fact_sources','report_source_failures')


def verify(request, sql, token, other_token):
    issued, cursors = [], []

    def snapshot():
        return {table: sql("SELECT coalesce(json_agg(t ORDER BY id)::text,'[]') FROM app."+table+' t') for table in TABLES}

    def make_user(count):
        uid = '0'+hashlib.sha256(('collection-jobs-owner-'+str(count)).encode()).hexdigest()[:25].upper()
        sid = '0'+hashlib.sha256(('collection-jobs-session-'+str(count)).encode()).hexdigest()[:25].upper()
        raw = secrets.token_bytes(32)
        sql("INSERT INTO app.users(id,login_id,password_hash) VALUES ('"+uid+"','verify_collection_"+str(count)+"',repeat('x',90)); INSERT INTO app.auth_sessions(id,user_id,token_hash) VALUES ('"+sid+"','"+uid+"',decode('"+hashlib.sha256(raw).hexdigest()+"','hex')); INSERT INTO app.collections(id,user_id,name,research_instruction,initial_lookback_mode,display_order) VALUES ('"+cid(uid,'selected')+"','"+uid+"','Collection履歴（架空）','架空','ai',0),('"+cid(uid,'empty')+"','"+uid+"','空Collection（架空）','架空','ai',1);")
        auth = base64.urlsafe_b64encode(raw).decode().rstrip('=')
        issued.append(auth)
        return uid, auth, cid(uid,'selected'), cid(uid,'empty')

    def add_history(uid, count, collection_id):
        _, rows = seed['fixture_rows'](uid, count)
        counter = 0
        for table, row in rows:
            if table == 'generation_jobs':
                row['collection_id'] = collection_id
                row['collection_name'] = 'Collection履歴（架空）'
                row['scheduled_at'] = '2026-04-01T15:00:00.'+f'{900000-counter:06d}'+'Z'
                counter += 1
        # Retain fixture Collections because fixture targets reference them; dedicated
        # selected/empty Collections remain separate. Jobs all point to selected.
        sql('BEGIN;'+''.join('INSERT INTO app.'+table+'('+','.join(row)+') VALUES ('+','.join(literal(v) for v in row.values())+');' for table,row in rows)+'COMMIT;')

    def cid(uid, purpose):
        return '0'+hashlib.sha256(('collection-jobs:'+uid+':'+purpose).encode()).hexdigest()[:25].upper()

    fixtures = {n: make_user(n) for n in (0,30,31,35)}
    for count, (uid, _, selected, _) in fixtures.items():
        if count:
            add_history(uid, count, selected)
    before = snapshot()

    def get(collection, auth, raw=''):
        path = '/api/collections/'+collection+'/generation-jobs'+(('?'+raw) if raw else '')
        status, headers, body = request(path, auth)
        assert headers.get('Cache-Control') == 'no-store'
        return status, headers, body

    for count, (uid, auth, selected, empty) in fixtures.items():
        status, _, body = get(selected, auth)
        page = json.loads(body)
        assert status == 200 and len(page['generation_jobs']) == min(count,30)
        assert (page['next_cursor'] is not None) == (count > 30)
        rows = page['generation_jobs']
        if page['next_cursor']:
            cursors.append(page['next_cursor'])
            status, _, tail_body = get(selected, auth, 'cursor='+page['next_cursor'])
            tail = json.loads(tail_body)
            assert status == 200 and len(tail['generation_jobs']) == count-30 and tail['next_cursor'] is None
            rows += tail['generation_jobs']
        assert [row['id'] for row in rows] == json.loads(sql("SELECT coalesce(json_agg(id ORDER BY scheduled_at DESC,id DESC)::text,'[]') FROM app.generation_jobs WHERE user_id='"+uid+"' AND collection_id='"+selected+"'"))
        assert len({row['id'] for row in rows}) == count
        assert all(set(row) == {'id','collection_name','scheduled_at','status','report_id'} for row in rows)
        if count:
            assert {row['status'] for row in rows} == {'queued','running','succeeded','failed'}
        assert json.loads(get(empty, auth)[2]) == {'generation_jobs': [], 'next_cursor': None}
        if count > 30:
            # A valid cursor from a different Collection remains constrained to this one.
            assert json.loads(get(empty, auth, 'cursor='+page['next_cursor'])[2]) == {'generation_jobs': [], 'next_cursor': None}

    foreign_id = fixtures[35][2]
    for auth in (token, other_token):
        foreign = get(foreign_id, auth)
        missing = get('0'+'0'*25, auth)
        assert foreign[0] == missing[0] == 404 and foreign[2] == missing[2]
        assert get(foreign_id, auth, 'cursor=%xx')[0] == 400
        assert get(foreign_id, None, 'cursor=%xx')[0] == 401
    assert snapshot() == before
    deleted_uid, deleted_auth, deleted_collection, _ = make_user(90)
    add_history(deleted_uid, 1, deleted_collection)
    selected = fixtures[35][2]
    success = next(row for row in json.loads(get(selected, fixtures[35][1])[2])['generation_jobs'] if row['status']=='succeeded')
    deleted_job = json.loads(get(deleted_collection, deleted_auth)[2])['generation_jobs'][0]
    assert deleted_job['status'] == 'succeeded' and deleted_job['report_id'] is not None
    before_delete = snapshot()
    sql("DELETE FROM app.collections WHERE id='"+deleted_collection+"'")
    after_delete = snapshot()
    deleted = get(deleted_collection, deleted_auth)
    missing_deleted = get('0'+'0'*25, deleted_auth)
    assert deleted[0] == missing_deleted[0] == 404 and deleted[2] == missing_deleted[2]
    daily = json.loads(request('/api/generation-jobs', deleted_auth)[2])
    assert deleted_job in daily['generation_jobs']
    assert snapshot() == after_delete
    assert success['report_id'] is not None
    assert request('/api/reports/'+success['report_id'], fixtures[35][1])[0] == 200
    before_jobs = json.loads(before_delete['generation_jobs'])
    after_jobs = json.loads(after_delete['generation_jobs'])
    for row in before_jobs:
        if row['id'] == deleted_job['id']:
            row['collection_id'] = None
    assert before_jobs == after_jobs
    assert all(before_delete[table] == after_delete[table] for table in TABLES if table != 'generation_jobs')
    print('Collection Jobs API: 0/30/31/35, mixed statuses, empty vs missing, collection scoped cursor, owner isolation, deleted history on daily API, auth priority and read-only history: passed', flush=True)
    return {'tokens': issued, 'cursors': cursors, 'identifiers': [v[2] for v in fixtures.values()] + [deleted_collection]}
