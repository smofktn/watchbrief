#!/usr/bin/env python3
"""Verify owner-scoped Report reads using fictional local fixtures only."""
from http.cookies import SimpleCookie
import json
import os
from pathlib import Path
import runpy
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
ORIGIN = "http://127.0.0.1:5173"


helpers = runpy.run_path(str(ROOT / 'scripts/verify-collections-api.py'))
query = helpers['query']
fixture = helpers['fixture']


def main():
    primary = fixture("user-create-verification.json", "verify_cli_")
    other = fixture("logout-verification.json", "verify_logout_")
    empty = fixture("reports-empty-verification.json", "verify_empty_", create=True)
    seed_module = runpy.run_path(str(ROOT / "scripts/seed-reports.py"))
    seed = seed_module['seed']
    ids = seed(primary["login_id"])
    other_ids = seed(other["login_id"])
    owner = query(f"SELECT user_id FROM app.generation_jobs WHERE id='{ids['job']}';").stdout.strip()
    _, expected_rows = seed_module['fixture_rows'](owner)

    def stored_fixture():
        # Include every fixture row and its timestamps: repeat must not edit any table.
        by_table = {}
        for table, row in expected_rows:
            by_table.setdefault(table, []).append(row['id'])
        fields = []
        for table, identifiers in by_table.items():
            literals = ','.join("'" + identifier + "'" for identifier in identifiers)
            fields.append(f"'{table}',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM app.{table} t WHERE id IN ({literals}))")
        result = query('SELECT jsonb_build_object(' + ','.join(fields) + ');')
        assert result.returncode == 0, 'Fixture snapshot read failed'
        return json.loads(result.stdout)

    def assert_saved_content(fixture_ids):
        result = query(f"""SELECT jsonb_build_object(
'job',(SELECT to_jsonb(j) FROM app.generation_jobs j WHERE id='{fixture_ids['job']}'),
'report',(SELECT to_jsonb(r) FROM app.reports r WHERE id='{fixture_ids['report']}'),
'result',(SELECT to_jsonb(t) FROM app.report_target_results t WHERE id='{fixture_ids['result']}'),
'facts',(SELECT jsonb_agg(to_jsonb(f) ORDER BY display_order) FROM app.report_facts f WHERE report_target_result_id='{fixture_ids['result']}'),
'sources',(SELECT jsonb_agg(to_jsonb(s)) FROM app.report_sources s WHERE report_target_result_id='{fixture_ids['result']}'),
'links',(SELECT jsonb_agg(to_jsonb(l)) FROM app.fact_sources l JOIN app.report_facts f ON f.id=l.report_fact_id WHERE f.report_target_result_id='{fixture_ids['result']}'),
'failure',(SELECT to_jsonb(f) FROM app.report_source_failures f WHERE id='{fixture_ids['failure']}')
);""")
        assert result.returncode == 0, 'Saved structured content lookup failed'
        document = json.loads(result.stdout)
        job, report, target = (document[key] for key in ('job', 'report', 'result'))
        snapshot = job['input_snapshot']
        assert snapshot['schema_version'] == 1 and snapshot['fixture'] == 'saved-report-v3'
        assert snapshot['research_instruction'] == '外部取得を行わない検証用。'
        assert snapshot['initial_lookback_mode'] == 'ai' and snapshot['initial_lookback_days'] is None
        assert not any(key in snapshot for key in ('cron_expression', 'password', 'token', 'secret', 'collection_name'))
        input_target, = snapshot['targets']
        assert input_target['id'] == target['target_id'] and input_target['name'] == target['target_name']
        assert input_target['instruction'] == '架空データだけで現在状態と更新を確認する。' and input_target['display_order'] == target['display_order']
        source, = document['sources']
        input_source, = input_target['sources']
        assert input_source['id'] == source['source_id'] and input_source['name'] == source['source_name'] and input_source['url'] == source['source_url']
        assert input_source['instruction'] == '外部へアクセスしない。'
        assert source['report_target_result_id'] == target['id']
        assert target['status'] == '重要な変化あり'
        assert {fact['fact_kind'] for fact in document['facts']} == {'current_state', 'update'}
        assert len(document['facts']) == 2 and len(document['links']) == 2
        body = report['body_markdown']
        assert target['status'] in body and target['target_name'] in body and target['ai_summary_markdown'] in body
        assert document['failure']['source_name'] in body and document['failure']['source_url'] in body
        assert document['failure']['reason'] and document['failure']['reason'] not in body, 'Failure reason must remain structured-only'
        assert '理由：' not in body
        for fact in document['facts']:
            assert fact['report_target_result_id'] == target['id']
            assert any(link['report_fact_id'] == fact['id'] and link['report_source_id'] == source['id'] for link in document['links']), 'Fact lacks same-target source'
            assert fact['content_markdown'] + '\n出典：' + source['source_url'] in body, 'Saved body differs from sourced facts'
            if fact['fact_kind'] == 'update':
                assert report['summary_markdown'] == fact['content_markdown']
        return document

    before = stored_fixture()
    seed(primary['login_id'])
    assert stored_fixture() == before, 'Seed repeat modified fixture rows'
    content = assert_saved_content(ids)
    assert content['job']['input_snapshot']['research_instruction'] == before['collections'][0]['research_instruction']
    assert content['job']['input_snapshot']['initial_lookback_mode'] == before['collections'][0]['initial_lookback_mode']
    assert content['job']['input_snapshot']['targets'][0]['id'] == before['watch_targets'][0]['id']
    assert content['job']['input_snapshot']['targets'][0]['sources'][0]['id'] == before['information_sources'][0]['id']
    # Deliberate fixture collision must leave all other inserts rolled back and never overwrite it.
    empty_owner = query(f"SELECT id FROM app.users WHERE login_id='{empty['login_id']}';").stdout.strip()
    conflict_ids, _ = seed_module['fixture_rows'](empty_owner)
    assert query(f"BEGIN; SET LOCAL ROLE watchbrief_migrator; INSERT INTO app.collections(id,user_id,name) VALUES('{conflict_ids['collection']}','{empty_owner}','Report seed collision verification') ON CONFLICT(id) DO NOTHING; COMMIT;").returncode == 0
    try:
        seed(empty['login_id'])
    except SystemExit:
        pass
    else:
        raise AssertionError('Conflicting seed overwrote existing content')
    assert query(f"SELECT count(*) FROM app.generation_jobs WHERE id='{conflict_ids['job']}';").stdout.strip() == '0', 'Failed seed partially persisted history'
    assert query(f"SELECT name FROM app.collections WHERE id='{conflict_ids['collection']}';").stdout.strip() == 'Report seed collision verification'
    with socket.socket() as reserved:
        reserved.bind(("127.0.0.1", 0))
        port = reserved.getsockname()[1]
    url = f"http://127.0.0.1:{port}"
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    issued = []

    def request(path, token=None, method="GET", payload=None):
        headers = {"Origin": ORIGIN}
        if token is not None:
            headers["Cookie"] = "watchbrief_session=" + token
        if payload is not None:
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(url + path, headers=headers, method=method, data=json.dumps(payload).encode() if payload is not None else None)
        try:
            response = client.open(req, timeout=15)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            return response.status, response.headers, response.read()

    def login(value):
        status, headers, _ = request("/api/login", method="POST", payload=value)
        assert status == 204, "Login failed"
        cookie = SimpleCookie()
        cookie.load(headers["Set-Cookie"])
        token = cookie["watchbrief_session"].value
        issued.append(token)
        return token

    with tempfile.TemporaryFile(mode="w+b", dir=ROOT / ".local") as logs:
        process = subprocess.Popen([str(ROOT / ".local/bin/api")], cwd=ROOT / "backend", env={**os.environ, "API_ADDR": f"127.0.0.1:{port}", "APP_ORIGIN": ORIGIN, "API_PASSWORD_FILE": str(ROOT / ".local/watchbrief-api-password")}, stdout=logs, stderr=logs)
        try:
            for _ in range(200):
                assert process.poll() is None, "API startup failed"
                if b'"msg":"api_started"' in os.pread(logs.fileno(), 1048576, 0):
                    break
                time.sleep(0.05)
            else:
                raise AssertionError("API readiness timed out")
            token = login(primary)
            other_token = login(other)
            empty_token = login(empty)
            for path in ("/api/reports", "/api/reports/" + ids["report"]):
                for invalid in (None, "invalid", token + "; watchbrief_session=invalid"):
                    status, headers, body = request(path, invalid)
                    assert status == 401 and json.loads(body)["error"]["code"] == "unauthenticated", "Unauthenticated Report exposed"
                    assert headers.get("Cache-Control") == "no-store"
            status, headers, body = request("/api/reports", token)
            rows = json.loads(body)["reports"]
            assert status == 200 and headers.get("Cache-Control") == "no-store"
            assert any(row["id"] == ids["report"] for row in rows)
            assert not any(row["id"] == other_ids["report"] for row in rows), "Other User leaked in list"
            status, _, body = request("/api/reports/" + ids["report"], token)
            value = json.loads(body)
            assert status == 200 and value["id"] == ids["report"] and "user_id" not in value
            assert "<script>" in value["body_markdown"]
            assert value["collection_name"] == "保存Report確認用（架空）"
            denied = request("/api/reports/" + other_ids["report"], token)
            missing = request("/api/reports/00000000000000000000000000", token)
            assert denied[0] == missing[0] == 404 and denied[2] == missing[2], "Other ownership revealed"
            assert request("/api/reports/" + ids["report"], other_token)[0] == 404
            assert request("/api/reports/invalid", token)[0] == 400
            assert request("/api/reports/invalid")[0] == 401
            status, _, body = request("/api/reports", empty_token)
            assert status == 200 and json.loads(body) == {"reports": []}, "Empty Report list is not []"
            for row in rows:
                assert set(row) == {"id", "collection_name", "scheduled_at", "generated_at"}
            print("Report v3 body/status/sourced current_state/update/snapshot agreement, seed repeat/collision rollback, own list/detail, saved body, empty arrays, 401/400 and indistinguishable owner/missing 404: passed")

            tables = ['generation_jobs', 'generation_attempts', 'reports', 'report_target_results', 'report_facts', 'report_sources', 'fact_sources', 'report_source_failures']
            for table in tables:
                assert query(f"BEGIN; SET LOCAL ROLE watchbrief_api; SELECT 1 FROM app.{table} LIMIT 0; ROLLBACK;").returncode == 0
                for operation in (f"DELETE FROM app.{table} WHERE false", f"UPDATE app.{table} SET id=id WHERE false", f"INSERT INTO app.{table}(id) VALUES('00000000000000000000000000')"):
                    result = query(f"BEGIN; SET LOCAL ROLE watchbrief_api; {operation}; ROLLBACK;")
                    assert result.returncode != 0 and '42501' in result.stderr, 'API got history write privilege'
                result = query(f"BEGIN; SET LOCAL ROLE watchbrief_user_creator; SELECT 1 FROM app.{table} LIMIT 0; ROLLBACK;")
                assert result.returncode != 0 and '42501' in result.stderr
            # Rollback-only mutation tests verify existing fictional history stays untouched.
            tests = [
                (f"UPDATE app.generation_attempts SET generation_job_id='{other_ids['job']}' WHERE id='{ids['attempt']}';", '23514'),
                (f"UPDATE app.reports SET generation_attempt_id='{other_ids['attempt']}' WHERE id='{ids['report']}';", '23514'),
                (f"UPDATE app.generation_jobs SET collection_id='{other_ids['collection']}' WHERE id='{ids['job']}';", '23514'),
                (f"UPDATE app.generation_jobs SET id='00000000000000000000000005' WHERE id='{ids['job']}';", '23514'),
                (f"UPDATE app.generation_attempts SET id='00000000000000000000000005' WHERE id='{ids['attempt']}';", '23514'),
                (f"UPDATE app.reports SET id='00000000000000000000000005' WHERE id='{ids['report']}';", '23514'),
                (f"UPDATE app.generation_jobs SET user_id=NULL WHERE id='{ids['job']}';", '23514'),
                (f"UPDATE app.generation_jobs SET input_snapshot='[]' WHERE id='{ids['job']}';", '23514'),
                (f"UPDATE app.generation_attempts SET status='running',finished_at=NULL WHERE id='{ids['attempt']}';", '23514'),
                (f"DELETE FROM app.reports WHERE id='{ids['report']}';", '23001'),
                (f"DELETE FROM app.generation_attempts WHERE id='{ids['attempt']}';", '23001'),
                (f"DELETE FROM app.generation_jobs WHERE id='{ids['job']}';", '23001'),
                (f"UPDATE app.generation_attempts SET finished_at='2025-01-01Z' WHERE id='{ids['attempt']}';", '23514'),
                (f"UPDATE app.report_sources SET source_id=NULL WHERE id='{ids['report_source']}';", '23514'),
                (f"UPDATE app.report_facts SET id='invalid' WHERE id='{ids['fact']}';", '23514'),
                (f"UPDATE app.report_target_results SET report_id='00000000000000000000000000' WHERE id='{ids['result']}';", '23503'),
                (f"UPDATE app.reports SET updates_started_at='2030-01-01Z' WHERE id='{ids['report']}';", '23514'),
                (f"UPDATE app.generation_attempts SET attempt_number=2 WHERE id='{ids['attempt']}';", '23514'),
                (f"INSERT INTO app.reports SELECT '00000000000000000000000001',generation_attempt_id,body_markdown,summary_markdown,updates_started_at FROM app.reports WHERE id='{ids['report']}';", '23505'),
            ]
            # A successful Attempt cannot commit without its Report.
            consistency_probe = f"""BEGIN; SET LOCAL ROLE watchbrief_migrator;
INSERT INTO app.generation_jobs SELECT '00000000000000000000000002',user_id,NULL,collection_name,input_snapshot,scheduled_at,queued_at,'succeeded' FROM app.generation_jobs WHERE id='{ids['job']}';
INSERT INTO app.generation_attempts SELECT '00000000000000000000000003','00000000000000000000000002',1,started_at,heartbeat_at,finished_at,'succeeded',NULL FROM app.generation_attempts WHERE id='{ids['attempt']}';
SET CONSTRAINTS ALL IMMEDIATE; ROLLBACK;"""
            result = query(consistency_probe)
            assert result.returncode != 0 and '23514' in result.stderr, 'Successful Attempt committed without Report'
            for index, (statement, code) in enumerate(tests):
                result = query('BEGIN; SET LOCAL ROLE watchbrief_migrator; ' + statement + 'SET CONSTRAINTS ALL IMMEDIATE; ROLLBACK;')
                assert result.returncode != 0 and code in result.stderr, f'History constraint missing at test {index}; expected {code}'
            for active_status in ('queued', 'running'):
                active = f"""BEGIN; SET LOCAL ROLE watchbrief_migrator;
INSERT INTO app.collections(id,user_id,name) SELECT '00000000000000000000000006',user_id,'Active deletion probe' FROM app.generation_jobs WHERE id='{ids['job']}';
INSERT INTO app.generation_jobs SELECT '00000000000000000000000007',user_id,'00000000000000000000000006',collection_name,input_snapshot,scheduled_at,queued_at,'{active_status}' FROM app.generation_jobs WHERE id='{ids['job']}';
"""
                if active_status == 'running':
                    active += f"INSERT INTO app.generation_attempts SELECT '00000000000000000000000008','00000000000000000000000007',1,started_at,heartbeat_at,NULL,'running',NULL FROM app.generation_attempts WHERE id='{ids['attempt']}';"
                null_input = f"""BEGIN; SET LOCAL ROLE watchbrief_migrator;
INSERT INTO app.generation_jobs SELECT '00000000000000000000000009',user_id,NULL,collection_name,input_snapshot,scheduled_at,queued_at,'{active_status}' FROM app.generation_jobs WHERE id='{ids['job']}'; ROLLBACK;"""
                result = query(null_input)
                assert result.returncode != 0 and '23514' in result.stderr, 'Active Job accepted without Collection'
                active += "SET CONSTRAINTS ALL IMMEDIATE; DELETE FROM app.collections WHERE id='00000000000000000000000006'; ROLLBACK;"
                result = query(active)
                assert result.returncode != 0 and '23514' in result.stderr, 'Active Collection deletion allowed'
            # Use a temporary dedicated Collection and committed successful history clone.
            # Deletion of its settings is real, so following API reads test authorization after SET NULL.
            owner = query(f"SELECT user_id FROM app.generation_jobs WHERE id='{ids['job']}';").stdout.strip()
            temporary = '0' + secrets.token_hex(13)[:25].upper()
            job = '0' + secrets.token_hex(13)[:25].upper()
            attempt = '0' + secrets.token_hex(13)[:25].upper()
            report = '0' + secrets.token_hex(13)[:25].upper()
            children = {kind: '0' + secrets.token_hex(13)[:25].upper() for kind in ('result','fact','current_fact','source','link','current_link','failure')}
            assert query(f"""BEGIN; SET LOCAL ROLE watchbrief_migrator;
INSERT INTO app.collections(id,user_id,name) SELECT '{temporary}','{owner}',collection_name FROM app.generation_jobs WHERE id='{ids['job']}';
INSERT INTO app.generation_jobs SELECT '{job}',user_id,'{temporary}',collection_name,jsonb_set(input_snapshot,'{{collection_id}}',to_jsonb('{temporary}'::text)),scheduled_at,queued_at,status FROM app.generation_jobs WHERE id='{ids['job']}';
INSERT INTO app.generation_attempts SELECT '{attempt}','{job}',attempt_number,started_at,heartbeat_at,finished_at,status,failure_type FROM app.generation_attempts WHERE id='{ids['attempt']}';
INSERT INTO app.reports SELECT '{report}','{attempt}',body_markdown,summary_markdown,updates_started_at FROM app.reports WHERE id='{ids['report']}';
INSERT INTO app.report_target_results SELECT '{children['result']}','{report}',target_id,target_name,display_order,status,ai_summary_markdown FROM app.report_target_results WHERE id='{ids['result']}';
INSERT INTO app.report_facts SELECT '{children['current_fact']}','{children['result']}',fact_kind,content_markdown,display_order FROM app.report_facts WHERE id='{ids['current_fact']}';
INSERT INTO app.report_facts SELECT '{children['fact']}','{children['result']}',fact_kind,content_markdown,display_order FROM app.report_facts WHERE id='{ids['fact']}';
INSERT INTO app.report_sources SELECT '{children['source']}','{children['result']}',source_id,source_kind,source_name,source_url FROM app.report_sources WHERE id='{ids['report_source']}';
INSERT INTO app.fact_sources VALUES('{children['current_link']}','{children['current_fact']}','{children['source']}');
INSERT INTO app.fact_sources VALUES('{children['link']}','{children['fact']}','{children['source']}');
INSERT INTO app.report_source_failures SELECT '{children['failure']}','{children['result']}',source_id,source_kind,source_name,source_url,reason FROM app.report_source_failures WHERE id='{ids['failure']}';
DELETE FROM app.collections WHERE id='{temporary}'; COMMIT;""").returncode == 0, 'History retention failed'
            assert query(f"SELECT collection_id IS NULL FROM app.generation_jobs WHERE id='{job}';").stdout.strip() == 't'
            assert_saved_content({'job': job, 'report': report, 'result': children['result'], 'failure': children['failure']})
            assert request('/api/reports/' + report, token)[0] == 200
            assert request('/api/reports/' + report, other_token)[0] == 404
            # Retain this run's fictional successful history, consistent with MVP retention.
            output = os.pread(logs.fileno(), 1048576, 0).decode()
            for secret in [primary["password"], other["password"], empty["password"], *issued]:
                assert secret not in output, "Private value in API logs"
            assert value["body_markdown"] not in output, "Report text in API logs"
            print("History SELECT-only grants, creator isolation, state/time/FK/unique/ULID constraints, Collection deletion retention/authorization and log privacy: passed")
        finally:
            for token in issued:
                try:
                    request("/api/logout", token, method="POST")
                except Exception:
                    pass
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=15)


if __name__ == "__main__":
    main()
