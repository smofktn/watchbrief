#!/usr/bin/env python3
"""Runs only inside a disposable database; all data and sessions are fictional."""
import base64
import hashlib
import json
from pathlib import Path
import runpy
import secrets

ROOT=Path(__file__).resolve().parent.parent
seed=runpy.run_path(str(ROOT/'scripts/seed-report-pages.py'))
literal=seed['original']['literal']
TABLES=('generation_jobs','generation_attempts','reports','report_target_results','report_facts','report_sources','fact_sources','report_source_failures')


def verify(request,sql,token,other_token):
    issued_cursors=set()
    def snapshot():
        return {t:sql("SELECT coalesce(json_agg(t ORDER BY id)::text,'[]') FROM app."+t+' t') for t in TABLES}
    def get(cursor=None,auth=token,raw=None):
        path='/api/reports'+(('?cursor='+cursor) if cursor else '')
        if raw is not None:path='/api/reports?'+raw
        status,_,body=request(path,auth)
        page=json.loads(body)
        if status==200 and page.get('next_cursor'):
            issued_cursors.add(page['next_cursor'])
        return status,page
    def insert(rows):
        sql('BEGIN;'+''.join('INSERT INTO app.'+t+'('+','.join(row)+') VALUES ('+','.join(literal(v) for v in row.values())+');' for t,row in rows)+'COMMIT;')
    def user(count):
        uid='0'+hashlib.sha256(('page-boundary-user-'+str(count)).encode()).hexdigest()[:25].upper()
        sid='0'+hashlib.sha256(('page-boundary-session-'+str(count)).encode()).hexdigest()[:25].upper()
        raw=secrets.token_bytes(32)
        sql("INSERT INTO app.users(id,login_id,password_hash) VALUES ('"+uid+"','verify_pages_"+str(count)+"',repeat('x',90)); INSERT INTO app.auth_sessions(id,user_id,token_hash) VALUES ('"+sid+"','"+uid+"',decode('"+hashlib.sha256(raw).hexdigest()+"','hex'));")
        _,rows=seed['fixture_rows'](uid,count)
        # All 31 rows tie exactly: the 30/31 boundary must use Report ID,
        # not just scheduled_at. Other users retain mixed microsecond cases.
        if count==31:
            for table,row in rows:
                if table=='generation_jobs':row['scheduled_at']='2026-04-01T00:00:00.000040Z'
        insert(rows)
        return uid,base64.urlsafe_b64encode(raw).decode().rstrip('=')
    boundary={count:user(count) for count in (0,30,31)}
    before=snapshot()
    for count,(_,auth) in boundary.items():
        status,page=get(auth=auth)
        assert status==200 and len(page['reports'])==min(count,30)
        assert (page['next_cursor'] is not None)==(count>30)
    status,first=get()
    assert status==200 and len(first['reports'])==30 and first['next_cursor']
    cursor=first['next_cursor']
    raw=base64.urlsafe_b64decode(cursor+'=')
    assert len(raw)==56 and raw.decode().startswith('1|2026-04-01T00:00:00.')
    status,last=get(cursor)
    assert status==200 and len(last['reports'])==5 and last['next_cursor'] is None
    assert get(cursor)==(status,last)
    ids=[r['id'] for r in first['reports']+last['reports']]
    assert len(ids)==len(set(ids))==35
    owner=sql("SELECT user_id FROM app.generation_jobs j JOIN app.generation_attempts a ON a.generation_job_id=j.id JOIN app.reports r ON r.generation_attempt_id=a.id WHERE r.id='"+ids[0]+"'")
    expected=json.loads(sql("SELECT json_agg(r.id ORDER BY j.scheduled_at DESC,r.id DESC)::text FROM app.reports r JOIN app.generation_attempts a ON a.id=r.generation_attempt_id JOIN app.generation_jobs j ON j.id=a.generation_job_id WHERE j.user_id='"+owner+"'"))
    assert ids==expected
    # Deliberately prove there is a sub-millisecond distinction opposing ID order.
    assert any(a['scheduled_at']!=b['scheduled_at'] and a['id']<b['id'] for a,b in zip(first['reports'],first['reports'][1:]))
    other=get(auth=other_token)[1]
    assert set(ids).isdisjoint(r['id'] for r in other['reports'])
    altered=get(other['next_cursor'])[1]
    assert all(r['id'] in ids for r in altered['reports'])
    assert get(raw='cursor=bad',auth=None)[0]==401
    for query in ('cursor=','cursor=bad','cursor=%xx','cursor='+cursor+'&cursor='+cursor,'limit=1000000','unknown=1','cursor='+cursor+'=','cursor='+cursor+'&limit=30','x'*513,';'):
        assert get(raw=query)[0]==400
    invalid_raw=base64.urlsafe_b64encode(raw.replace(b'1|',b'2|',1)).decode().rstrip('=')
    assert get(invalid_raw)[0]==400
    for at,length in [('0001-01-01T00:00:00.000000Z',0),('9999-01-01T00:00:00.000000Z',30)]:
        boundary_cursor=base64.urlsafe_b64encode(('1|'+at+'|'+'0'*26).encode()).decode().rstrip('=')
        assert len(get(boundary_cursor)[1]['reports'])==length
    assert snapshot()==before
    # Insert a newer Report for an independent 31-row user; an old continuation
    # still returns the old last row, while reloading sees the new head.
    uid,auth=boundary[31]
    old=get(auth=auth)[1]
    _,rows=seed['fixture_rows'](uid+'-new-head',1)
    for table,row in rows:
        if table in ('collections','generation_jobs'):row['user_id']=uid
        if table=='generation_jobs':row['scheduled_at']='2027-01-01T00:00:00.123456Z'
    new_id=next(row['id'] for table,row in rows if table=='reports')
    insert(rows)
    after_insert=snapshot()
    assert get(auth=auth)[1]['reports'][0]['id']==new_id
    tail=get(old['next_cursor'],auth=auth)[1]
    assert len(tail['reports'])==1 and tail['reports'][0]['id']!=new_id
    assert snapshot()==after_insert
    # Deleted Collection history remains owned through Job; no app deletion API.
    uid,auth=boundary[30]
    kept=get(auth=auth)[1]['reports'][0]['id']
    cid=sql("SELECT collection_id FROM app.generation_jobs j JOIN app.generation_attempts a ON a.generation_job_id=j.id JOIN app.reports r ON r.generation_attempt_id=a.id WHERE r.id='"+kept+"'")
    sql("DELETE FROM app.collections WHERE id='"+cid+"'")
    after_delete=snapshot()
    assert kept in [r['id'] for r in get(auth=auth)[1]['reports']]
    assert request('/api/reports/'+kept,auth)[0]==200
    foreign=request('/api/reports/'+kept,token)
    missing=request('/api/reports/'+'0'*26,token)
    assert foreign[0]==missing[0]==404 and foreign[2]==missing[2]
    # Expired session dominates malformed query, too.
    assert request('/api/logout',auth,{},method='POST')[0]==204
    assert get(raw='cursor=%xx',auth=auth)[0]==401
    assert snapshot()==after_delete
    print('Report pages API: 0/30/31/35, exact microsecond/tie order, 30+5/replay, insertion without offset duplicate, cursor/query rejection and auth priority, owner isolation/deleted Collection history and read-only data: passed',flush=True)

    return {'tokens':[value[1] for value in boundary.values()], 'cursors':list(issued_cursors)}
