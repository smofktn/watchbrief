#!/usr/bin/env python3
"""Runs only inside the disposable project's isolated database and API."""
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import uuid


def verify(request, sql, owner_token, other_token, seeded_id):
    fields = ('name', 'research_instruction', 'initial_lookback_mode', 'initial_lookback_days', 'version')
    initial = dict(name='架空Schedule検証', research_instruction='', initial_lookback_mode='ai', initial_lookback_days=None)
    status, _, raw = request('/api/collections', owner_token, initial, method='POST', extra_headers={'Idempotency-Key': str(uuid.uuid4())})
    assert status == 201
    cid = json.loads(raw)['id']
    path = '/api/collections/' + cid
    def detail():
        status, _, body = request(path, owner_token)
        assert status == 200
        return json.loads(body)
    def base():
        return {k: detail()[k] for k in fields}
    def patch(body, key=None, token=owner_token, **kw):
        return request(path, token, body, method='PATCH', extra_headers={'Idempotency-Key': key or str(uuid.uuid4())}, **kw)
    tables = ('collections','watch_targets','information_sources','collection_schedules','collection_update_requests','collection_creation_requests','generation_jobs','generation_attempts','reports','report_target_results','report_facts','report_sources','fact_sources','report_source_failures')
    def snapshot():
        return json.loads(sql('SELECT json_build_object(' + ','.join("'"+t+"',(SELECT coalesce(json_agg(t ORDER BY to_jsonb(t)::text)::text,'[]') FROM app."+t+' t)' for t in tables) + ')::text'))
    stable = snapshot()
    assert detail()['schedule'] is None
    # No schedule remains valid; install an exact pre-extension source digest.
    assert patch({**base(), 'targets':[dict(id=None,name='架空対象',instruction='')]})[0] == 200
    tid = detail()['targets'][0]['id']
    source = dict(id=None,watch_target_id=tid,name='架空情報源',url='https://example.com',instruction='')
    oldbody = {**base(),'sources':[source]}
    oldkey = str(uuid.uuid4())
    assert patch(oldbody, oldkey)[0] == 200
    expected = hashlib.sha256(json.dumps(dict(id=cid,**oldbody),ensure_ascii=False,separators=(',',':')).encode()).hexdigest()
    assert sql("SELECT encode(input_digest,'hex') FROM app.collection_update_requests WHERE request_key='"+oldkey+"'") == expected
    assert detail()['schedule'] is None
    add = dict(id=None,cron_expression=' 0 7,19 * * *\t')
    body = {**base(),'name':'Scheduleと本体同時保存','targets':[dict(id=tid,name='同時保存対象',instruction='補足')],'sources':[source],'schedule':add}
    key = str(uuid.uuid4())
    # A failure after all settings (including a new Schedule) must leave no partial rows.
    settled=snapshot()
    sql("""CREATE FUNCTION app.verify_schedule_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fictional schedule failure'; END $$;
CREATE TRIGGER verify_schedule_failure BEFORE INSERT ON app.collection_update_requests FOR EACH ROW EXECUTE FUNCTION app.verify_schedule_failure();""")
    try:
        assert patch(body,key)[0]==500 and snapshot()==settled
    finally:
        sql('DROP TRIGGER verify_schedule_failure ON app.collection_update_requests; DROP FUNCTION app.verify_schedule_failure();')
    # 8 simultaneous first requests create one schedule, one request and one revision.
    with ThreadPoolExecutor(max_workers=8) as pool:
        replies = list(pool.map(lambda _:patch(body,key), range(8)))
    assert all(r[0] == 200 and r[2] == replies[0][2] for r in replies)
    saved = detail()
    sid = saved['schedule']['id']
    assert saved['schedule']['cron_expression'] == add['cron_expression'] and not saved['enabled']
    assert int(saved['version']) == int(body['version'])+1 and saved['targets'][0]['name']=='同時保存対象'
    assert len(saved['targets'][0]['sources']) == 2
    settled = snapshot()
    assert patch(oldbody,oldkey)[0] == 200 and snapshot() == settled
    assert patch({**body,'schedule':{**add,'cron_expression':'0 8 * * *'}},key)[0]==409 and snapshot()==settled
    assert patch({**base(),'schedule':add})[0]==409 and snapshot()==settled
    edit = dict(id=sid,cron_expression='*/30 7-19/2 * * 1-5')
    editbody={**base(),'schedule':edit}
    editkey=str(uuid.uuid4())
    with ThreadPoolExecutor(max_workers=8) as pool:
        edited=list(pool.map(lambda _:patch(editbody,editkey),range(8)))
    assert all(r[0]==200 and r[2]==edited[0][2] for r in edited)
    assert detail()['schedule']==edit
    settled=snapshot()
    assert patch(body,key)[0]==200 and patch(oldbody,oldkey)[0]==200 and snapshot()==settled
    # Omitted schedule preserves all its columns even when saving other settings.
    row = sql("SELECT to_jsonb(s)::text FROM app.collection_schedules s WHERE id='"+sid+"'")
    assert patch(base())[0]==200
    assert sql("SELECT to_jsonb(s)::text FROM app.collection_schedules s WHERE id='"+sid+"'")==row
    settled=snapshot()
    for expression in ('', ' ', '@daily', '0 0 7 * * *', '0 24 * * *', '0 7 * * MON', '0 7 * * *\n', 'TZ=UTC 0 7 * * *', 'x'*257):
        assert patch({**base(),'schedule':dict(id=sid,cron_expression=expression)})[0]==400 and snapshot()==settled
    seed = json.loads(request('/api/collections/'+seeded_id,owner_token)[2])
    if seed['schedule'] is None:
        assert request('/api/collections/'+seeded_id,owner_token,{**{k:seed[k] for k in fields},'schedule':dict(id=None,cron_expression='0 7 * * *')},method='PATCH',extra_headers={'Idempotency-Key':str(uuid.uuid4())})[0]==200
        seed = json.loads(request('/api/collections/'+seeded_id,owner_token)[2])
    foreign = seed['schedule']['id']
    othercid = json.loads(request('/api/collections',other_token)[2])['collections'][0]['id']
    other = json.loads(request('/api/collections/'+othercid,other_token)[2])
    if other['schedule'] is None:
        assert request('/api/collections/'+othercid,other_token,{**{k:other[k] for k in fields},'schedule':dict(id=None,cron_expression='0 7 * * *')},method='PATCH',extra_headers={'Idempotency-Key':str(uuid.uuid4())})[0]==200
        other = json.loads(request('/api/collections/'+othercid,other_token)[2])
    other_schedule = other['schedule']['id']
    settled=snapshot()
    hidden=[]
    for bad in (foreign,other_schedule,'00000000000000000000000000'):
        r=patch({**base(),'name':'巻き戻す名前','targets':[dict(id=tid,name='巻き戻す対象',instruction='')],'sources':[source],'schedule':dict(id=bad,cron_expression='0 8 * * *')})
        assert r[0]==404 and snapshot()==settled
        hidden.append(r[2])
    assert len(set(hidden))==1
    assert patch({**base(),'schedule':edit},token=other_token)[0]==404 and snapshot()==settled
    assert patch({**base(),'schedule':edit},token=None)[0]==401
    for origin in ('','null','https://external.invalid'):
        assert patch({**base(),'schedule':edit},supplied_origin=origin)[0]==403 and snapshot()==settled
    sql("UPDATE app.collections SET enabled=true WHERE id='"+cid+"'")
    settled=snapshot()
    assert patch({**base(),'schedule':edit})[0]==409 and patch(body,key)[0]==200 and snapshot()==settled
    sql("UPDATE app.collections SET enabled=false WHERE id='"+cid+"'")
    # Fail after schedule write: every body/target/source/schedule/version/request rolls back.
    failing={**base(),'name':'巻き戻す本体','targets':[dict(id=tid,name='巻き戻す対象',instruction='')],'sources':[source],'schedule':dict(id=sid,cron_expression='0 9 * * *')}
    failkey=str(uuid.uuid4())
    settled=snapshot()
    sql("""CREATE FUNCTION app.verify_schedule_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fictional schedule failure'; END $$;
CREATE TRIGGER verify_schedule_failure BEFORE INSERT ON app.collection_update_requests FOR EACH ROW EXECUTE FUNCTION app.verify_schedule_failure();""")
    try:
        assert patch(failing,failkey)[0]==500 and snapshot()==settled
    finally:
        sql('DROP TRIGGER verify_schedule_failure ON app.collection_update_requests; DROP FUNCTION app.verify_schedule_failure();')
    assert patch(failing,failkey)[0]==200
    # Each direction of a settings/Schedule race rejects stale whole drafts.
    b=base()
    assert patch({**b,'name':'先行本体保存'})[0]==200
    settled=snapshot()
    assert patch({**b,'schedule':edit})[0]==409 and snapshot()==settled
    b=base()
    assert patch({**b,'schedule':edit})[0]==200
    settled=snapshot()
    assert patch({**b,'name':'古い本体保存'})[0]==409 and snapshot()==settled
    # Distinct simultaneous mixed saves all share the Collection revision.
    b=base()
    with ThreadPoolExecutor(max_workers=8) as pool:
        replies=list(pool.map(lambda i:patch({**b,**({'schedule':dict(id=sid,cron_expression='0 '+str(i)+' * * *')} if i%2 else {'name':'先行本体'+str(i)})}),range(8)))
    assert sum(r[0]==200 for r in replies)==1 and sum(r[0]==409 for r in replies)==7
    assert all(snapshot()[t]==stable[t] for t in tables if t not in ('collections','watch_targets','information_sources','collection_schedules','collection_update_requests'))
    assert sql("SELECT count(*) FROM app.collection_schedules WHERE collection_id='"+cid+"'")=='1'
    for statement in ("INSERT INTO app.collection_schedules(id,collection_id,cron_expression,created_at) SELECT id,collection_id,cron_expression,now() FROM app.collection_schedules WHERE false",'DELETE FROM app.collection_schedules WHERE false','UPDATE app.collection_schedules SET collection_id=id WHERE false','UPDATE app.collection_schedules SET id=collection_id WHERE false','UPDATE app.collection_schedules SET created_at=now() WHERE false','UPDATE app.generation_jobs SET collection_name=\'x\' WHERE false'):
        sql('BEGIN; SET LOCAL ROLE watchbrief_api; DO $verify$ BEGIN BEGIN '+statement+"; RAISE EXCEPTION 'Unexpectedly allowed'; EXCEPTION WHEN insufficient_privilege THEN NULL; END; END $verify$; ROLLBACK;")
    print('Schedule API: add/edit/reload, unset/omission, atomic body/targets/sources, 8-way first-add replay and mixed update conflict, exact legacy source digest/replay, owner/parent 404/CSRF, cron validation, rollback/recovery, history and minimum privileges: passed',flush=True)
