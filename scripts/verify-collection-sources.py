#!/usr/bin/env python3
"""Only invoked inside the disposable verification project with fictional data."""
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import uuid


def verify(request, sql, owner_token, other_token, seeded_id):
    fields = ('name', 'research_instruction', 'initial_lookback_mode', 'initial_lookback_days', 'version')
    def detail(cid=seeded_id, token=owner_token):
        status, _, body = request('/api/collections/' + cid, token)
        assert status == 200
        return json.loads(body)
    def base():
        return {k: detail()[k] for k in fields}
    def patch(body, key=None, **kw):
        return request('/api/collections/' + seeded_id, kw.pop('token', owner_token), body, method='PATCH', extra_headers={'Idempotency-Key': key or str(uuid.uuid4())}, **kw)
    tables = ('collections','watch_targets','information_sources','collection_schedules','collection_update_requests','collection_creation_requests','generation_jobs','generation_attempts','reports','report_target_results','report_facts','report_sources','fact_sources','report_source_failures')
    def snapshot():
        return json.loads(sql('SELECT json_build_object(' + ','.join("'"+t+"',(SELECT coalesce(json_agg(t ORDER BY to_jsonb(t)::text)::text,'[]') FROM app."+t+' t)' for t in tables) + ')::text'))
    target = detail()['targets'][0]
    tid = target['id']
    source = dict(id=None, watch_target_id=tid, name='架空の公式情報源', url='https://EXAMPLE.com/news?q=日本語#発表', instruction='改行\n架空の指示')
    stable = snapshot()
    # Install a genuine legacy target request digest in the pre-extension format.
    oldbody = {**base(), 'targets':[dict(id=tid,name='旧対象要求の編集',instruction='旧指示')]}
    oldkey = str(uuid.uuid4())
    assert patch(oldbody, oldkey)[0] == 200
    digest_body = dict(id=seeded_id, **oldbody)
    old_digest = hashlib.sha256(json.dumps(digest_body,ensure_ascii=False,separators=(',',':')).encode()).hexdigest()
    assert sql("SELECT encode(input_digest,'hex') FROM app.collection_update_requests WHERE request_key='"+oldkey+"'") == old_digest
    # Body+target+source are one version and one transaction. Duplicate URLs are allowed.
    body = {**base(), 'name':'情報源と本体の同時保存', 'targets':[dict(id=tid,name='情報源と対象の同時保存',instruction='新指示')], 'sources':[source, {**source,'name':'同じURLの別登録'}]}
    key = str(uuid.uuid4())
    result = patch(body,key)
    assert result[0] == 200
    saved = detail()
    assert int(saved['version']) == int(body['version'])+1
    sources = next(t for t in saved['targets'] if t['id']==tid)['sources']
    new = next(s for s in sources if s['name']==source['name'])
    assert new['url']==source['url'] and new['instruction']==source['instruction']
    assert len(sources)==len(target['sources'])+2
    settled = snapshot()
    with ThreadPoolExecutor(max_workers=8) as pool:
        replies = list(pool.map(lambda _:patch(body,key),range(8)))
    assert all(r[0]==200 and r[2]==result[2] for r in replies) and snapshot()==settled
    assert patch({**body,'sources':[source]},key)[0]==409 and snapshot()==settled
    # Old target digest, old target-only replay, body-only stale version and source replay after later edit.
    assert patch(oldbody,oldkey)[0]==200 and snapshot()==settled
    assert patch({k:oldbody[k] for k in fields})[0]==409 and snapshot()==settled
    edit = {**source,'id':new['id'],'name':'編集後の架空情報源','url':'http://example.com/edited','instruction':''}
    assert patch({**base(),'sources':[edit]})[0]==200
    settled = snapshot()
    assert patch(body,key)[0]==200 and snapshot()==settled
    assert patch(oldbody,oldkey)[0]==200 and snapshot()==settled
    now = detail()
    new_target = next(t for t in now['targets'] if t['id']!=tid)
    # Hidden or mismatched parent and source IDs all have identical 404; no partial body/target writes.
    othercid = json.loads(request('/api/collections',other_token)[2])['collections'][0]['id']
    foreign = detail(othercid,other_token)['targets'][0]
    other_own = next(c['id'] for c in json.loads(request('/api/collections',owner_token)[2])['collections'] if c['id']!=seeded_id)
    own_target = detail(other_own)['targets'][0] if detail(other_own)['targets'] else None
    if own_target is None:
        c = detail(other_own)
        assert request('/api/collections/'+other_own,owner_token,{**{k:c[k] for k in fields},'targets':[dict(id=None,name='所属検証対象',instruction='')]},method='PATCH',extra_headers={'Idempotency-Key':str(uuid.uuid4())})[0]==200
        own_target=detail(other_own)['targets'][0]
    settled = snapshot()
    hidden=[]
    for change in ({**source,'watch_target_id':foreign['id']}, {**source,'watch_target_id':own_target['id']}, {**source,'watch_target_id':'00000000000000000000000000'}, {**source,'id':'00000000000000000000000000'}, {**edit,'watch_target_id':new_target['id']}, {**source,'id':foreign['sources'][0]['id']}):
        r=patch({**base(),'name':'巻き戻す名前','targets':[dict(id=tid,name='巻き戻す対象',instruction='')],'sources':[change]})
        assert r[0]==404 and snapshot()==settled
        hidden.append(r[2])
    assert len(set(hidden))==1
    for changes in ([{**source,'name':'　'}], [{**source,'name':'x'*201}], [{**source,'instruction':'x'*10001}], [{**source,'name':'x\0'}], [edit,edit], [source]*21):
        assert patch({**base(),'sources':changes})[0]==400 and snapshot()==settled
    for url in ('javascript:alert(1)','https://u:p@example.com','https://@example.com','//example.com','https://example.com:0','https://example.com/%xx','https://example.com/?q=%','https://example.com/ a','https://example.com/\n','https://example.com\\x','https://例.jp/'):
        assert patch({**base(),'sources':[{**source,'url':url}]})[0]==400 and snapshot()==settled
    # Valid boundary values, text (not markup), URL input string preserved.
    boundary={**source,'name':'あ'*200,'url':'https://example.com/'+('x'*(2048-len('https://example.com/'))),'instruction':'あ'*10000}
    assert patch({**base(),'sources':[boundary]})[0]==200
    settled=snapshot()
    assert patch({**base(),'sources':[{**boundary,'url':boundary['url']+'x'}]})[0]==400 and snapshot()==settled
    assert patch({**base(),'sources':[source]},token=None)[0]==401
    for origin in ('','null','https://external.invalid'):
        assert patch({**base(),'sources':[source]},supplied_origin=origin)[0]==403
    # No ON editing, but old successful requests still replay without mutating.
    sql("UPDATE app.collections SET enabled=true WHERE id='"+seeded_id+"'")
    settled=snapshot()
    assert patch({**base(),'sources':[source]})[0]==409 and patch(body,key)[0]==200 and snapshot()==settled
    sql("UPDATE app.collections SET enabled=false WHERE id='"+seeded_id+"'")
    # Failure after an added and edited source (request record trigger) rolls everything back.
    failing={**base(),'name':'巻き戻す本体','targets':[dict(id=tid,name='巻き戻す対象',instruction='')],'sources':[edit,source]}
    failkey=str(uuid.uuid4())
    settled=snapshot()
    sql("""CREATE FUNCTION app.verify_source_failure() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'fictional source failure'; END $$;
CREATE TRIGGER verify_source_failure BEFORE INSERT ON app.collection_update_requests FOR EACH ROW EXECUTE FUNCTION app.verify_source_failure();""")
    try:
        assert patch(failing,failkey)[0]==500 and snapshot()==settled
    finally:
        sql('DROP TRIGGER verify_source_failure ON app.collection_update_requests; DROP FUNCTION app.verify_source_failure();')
    assert patch(failing,failkey)[0]==200
    # Source and body/target saves race on the same whole revision: exactly one succeeds.
    b=base()
    with ThreadPoolExecutor(max_workers=8) as pool:
        replies=list(pool.map(lambda i:patch({**b,**({'sources':[{**source,'name':'並列情報源'+str(i)}]} if i%2 else {'targets':[dict(id=None,name='並列対象'+str(i),instruction='')]} )}),range(8)))
    assert sum(r[0]==200 for r in replies)==1 and sum(r[0]==409 for r in replies)==7
    # 20 source changes + 20 targets in a single request; total registration count may exceed 20.
    large={**base(),'sources':[{**source,'name':'多数情報源'+str(i),'instruction':'x'*10000} for i in range(20)],'targets':[dict(id=None,name='多数対象'+str(i),instruction='x'*10000) for i in range(20)]}
    assert patch(large)[0]==200
    assert len(next(t for t in detail()['targets'] if t['id']==tid)['sources'])>20
    assert all(snapshot()[t]==stable[t] for t in tables if t not in ('collections','watch_targets','information_sources','collection_update_requests'))
    for statement in ('DELETE FROM app.information_sources WHERE false','UPDATE app.information_sources SET watch_target_id=id WHERE false','UPDATE app.information_sources SET id=watch_target_id WHERE false','UPDATE app.generation_jobs SET collection_name=\'x\' WHERE false'):
        sql('BEGIN; SET LOCAL ROLE watchbrief_api; DO $verify$ BEGIN BEGIN '+statement+"; RAISE EXCEPTION 'Unexpectedly allowed'; EXCEPTION WHEN insufficient_privilege THEN NULL; END; END $verify$; ROLLBACK;")
    print('Source API: atomic body/target/source add/edit, duplicate URL, 8-way replay and mixed race, exact legacy digest/replay, owner/parent 404, URL/limits/CSRF, rollback/retry, history and minimum privileges: passed',flush=True)
