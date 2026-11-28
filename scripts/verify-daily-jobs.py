#!/usr/bin/env python3
"""Read-only API verification against only the outer runner's disposable DB."""
import base64
import hashlib
import json
from pathlib import Path
import runpy
import secrets

ROOT = Path(__file__).resolve().parent.parent
seed = runpy.run_path(str(ROOT/'scripts/seed-daily-jobs.py'))
literal = seed['original']['literal']
TABLES = ('generation_jobs','generation_attempts','reports','report_target_results','report_facts','report_sources','fact_sources','report_source_failures')


def verify(request, sql, token, other_token):
    cursors = set()
    tokens = []
    def snapshot():
        return {t: sql("SELECT coalesce(json_agg(t ORDER BY id)::text,'[]') FROM app."+t+' t') for t in TABLES}
    def get(cursor=None, auth=token, raw=None):
        path = '/api/generation-jobs'+(('?cursor='+cursor) if cursor else '')
        if raw is not None:
            path = '/api/generation-jobs?'+raw
        status, headers, body = request(path, auth)
        assert headers['Cache-Control'] == 'no-store'
        page = json.loads(body)
        if status == 200 and page['next_cursor']:
            cursors.add(page['next_cursor'])
        return status, page
    def user(count):
        uid = '0'+hashlib.sha256(('daily-boundary-user-'+str(count)).encode()).hexdigest()[:25].upper()
        sid = '0'+hashlib.sha256(('daily-boundary-session-'+str(count)).encode()).hexdigest()[:25].upper()
        raw = secrets.token_bytes(32)
        sql("INSERT INTO app.users(id,login_id,password_hash) VALUES ('"+uid+"','verify_daily_"+str(count)+"',repeat('x',90)); INSERT INTO app.auth_sessions(id,user_id,token_hash) VALUES ('"+sid+"','"+uid+"',decode('"+hashlib.sha256(raw).hexdigest()+"','hex'));")
        _, rows = seed['fixture_rows'](uid, count)
        if count == 31:
            for table, row in rows:
                if table == 'generation_jobs':
                    row['scheduled_at'] = '2026-04-01T15:00:00.123456Z'
        sql('BEGIN;'+''.join('INSERT INTO app.'+t+'('+','.join(row)+') VALUES ('+','.join(literal(v) for v in row.values())+');' for t,row in rows)+'COMMIT;')
        auth = base64.urlsafe_b64encode(raw).decode().rstrip('=')
        tokens.append(auth)
        return uid, auth
    boundaries = {n: user(n) for n in (0,30,31,35)}
    before = snapshot()
    for n, (uid, auth) in boundaries.items():
        status, first = get(auth=auth)
        assert status == 200 and len(first['generation_jobs']) == min(n,30)
        assert (first['next_cursor'] is not None) == (n>30)
        rows = first['generation_jobs']
        if n > 30:
            status, tail = get(first['next_cursor'], auth)
            assert status == 200 and len(tail['generation_jobs']) == n-30 and tail['next_cursor'] is None
            assert get(first['next_cursor'], auth) == (status, tail)
            rows += tail['generation_jobs']
        ids = [row['id'] for row in rows]
        expected = json.loads(sql("SELECT coalesce(json_agg(id ORDER BY scheduled_at DESC,id DESC)::text,'[]') FROM app.generation_jobs WHERE user_id='"+uid+"'"))
        assert ids == expected and len(set(ids)) == n
        for row in rows:
            assert set(row) == {'id','collection_name','scheduled_at','status','report_id'}
            assert (row['report_id'] is not None) == (row['status']=='succeeded')
            if row['report_id']:
                assert request('/api/reports/'+row['report_id'],auth)[0] == 200
        if n:
            assert {row['status'] for row in rows} == {'queued','running','succeeded','failed'}
        if n == 35:
            assert any(a['scheduled_at'] != b['scheduled_at'] and a['id'] < b['id'] for a,b in zip(rows,rows[1:]))
    status, first = get()
    assert status == 200 and len(first['generation_jobs']) == 30
    cursor = first['next_cursor']
    assert len(cursor) == 79 and base64.urlsafe_b64decode(cursor+'=').startswith(b'job1|')
    _, tail = get(cursor)
    ids = [r['id'] for r in first['generation_jobs']+tail['generation_jobs']]
    assert len(ids) == len(set(ids)) == 35
    other = get(auth=other_token)[1]
    assert set(ids).isdisjoint(row['id'] for row in other['generation_jobs'])
    assert all(row['id'] in ids for row in get(other['next_cursor'])[1]['generation_jobs'])
    # Both directions are explicitly tested with a syntactically valid foreign cursor.
    wrong_type = base64.urlsafe_b64encode(('1|2026-04-01T15:00:00.123456Z|'+'0'*26).encode()).decode().rstrip('=')
    assert get(wrong_type)[0] == 400
    assert request('/api/reports?cursor='+cursor,token)[0] == 400
    for raw in ('cursor=','cursor=bad','cursor=%xx','cursor='+cursor+'&cursor='+cursor,'limit=30','unknown=1','cursor='+cursor+'=','x'*513,';','cursor='+wrong_type):
        assert get(raw=raw)[0] == 400
        assert get(raw=raw,auth=None)[0] == 401
    assert snapshot() == before
    # Delete only a terminal Collection, retaining Job history and owner authorization.
    uid, auth = boundaries[30]
    row = next(r for r in get(auth=auth)[1]['generation_jobs'] if r['status']=='succeeded')
    cid = sql("SELECT collection_id FROM app.generation_jobs WHERE id='"+row['id']+"'")
    sql("DELETE FROM app.collections WHERE id='"+cid+"'")
    after_delete = snapshot()
    assert row in get(auth=auth)[1]['generation_jobs']
    assert request('/api/reports/'+row['report_id'],auth)[0] == 200
    foreign = request('/api/reports/'+row['report_id'],token)
    missing = request('/api/reports/'+'0'*26,token)
    assert foreign[0] == missing[0] == 404 and foreign[2] == missing[2]
    assert request('/api/logout',auth,{},method='POST')[0] == 204
    assert get(raw='cursor=%xx',auth=auth)[0] == 401
    assert snapshot() == after_delete
    print('Daily Jobs API: mixed states, 0/30/31/35, microsecond/exact tie boundary, owners/deleted Collection, cursor types/query/auth priority, safe detail and read-only history: passed',flush=True)
    return {'tokens': tokens, 'cursors': list(cursors)}
