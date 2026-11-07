#!/usr/bin/env python3
"""Fresh disposable DB, actual outage/recovery and #17 end-to-end verification.
Never operates on the normal Compose project or its volume. Secrets stay private.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import runpy
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from http.cookies import SimpleCookie

ROOT = Path(__file__).resolve().parent.parent


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def main():
    os.umask(0o077)
    with tempfile.TemporaryDirectory(prefix='watchbrief-issue17-') as directory:
        stage = Path(directory)
        project = 'watchbrief-verify-' + secrets.token_hex(8)
        db_port = free_port()
        if db_port == 5432:
            raise SystemExit('Refusing the normal DB port.')
        environment = {**os.environ, 'COMPOSE_PROJECT_NAME': project, 'DB_PORT': str(db_port)}
        # Run the normal source paths against private credentials and a fresh volume.
        shutil.copytree(ROOT / 'scripts', stage / 'scripts', ignore=shutil.ignore_patterns('__pycache__'))
        shutil.copy(ROOT / 'compose.yaml', stage / 'compose.yaml')
        (stage / 'backend').mkdir()
        (stage / 'backend/db').symlink_to(ROOT / 'backend/db', target_is_directory=True)
        (stage / '.local/bin').mkdir(parents=True)
        for name in ('api', 'user-create', 'password-reset', 'goose'):
            (stage / '.local/bin' / name).symlink_to(ROOT / '.local/bin' / name)
        environment['COMPOSE_FILE'] = str(stage / 'compose.yaml')
        # Resolve asdf in the real repo; the private fixture has no runtime config.
        node_result = subprocess.run(['node', '-p', 'process.execPath'], cwd=ROOT, env=environment,
                                     capture_output=True, text=True, timeout=15)
        if node_result.returncode or not Path(node_result.stdout.strip()).is_file():
            raise SystemExit('Node runtime unavailable; run make runtime-install first.')
        node_binary = node_result.stdout.strip()

        def run(arguments, content=None, expected=0):
            result = subprocess.run(arguments, input=content, cwd=stage, env=environment,
                                    text=True, capture_output=True, timeout=300)
            if result.returncode != expected:
                stage_label = ' '.join(line for line in result.stderr.splitlines() if (line.startswith('browser_stage=') or line.startswith('browser_code=')) and all(c.isalnum() or c in '_=-' for c in line))
                raise AssertionError('Verification command failed: ' + Path(arguments[0]).name + ' ' + stage_label + ' (private output suppressed)')
            return result.stdout

        def script(name, *args):
            return run(['python3', str(stage / 'scripts' / name), *args])

        def compose(*args):
            return run(['docker', 'compose', *args])

        def sql(statement):
            return run(['docker', 'compose', 'exec', '-T', 'postgres', 'psql', '-X', '-qAt', '-U', 'postgres',
                        '-d', 'watchbrief', '-v', 'ON_ERROR_STOP=1'], statement).strip()

        def normal_db_state():
            result = run(['docker', 'ps', '-a', '--filter', 'label=com.docker.compose.project=watchbrief-local', '--format', '{{.ID}}'])
            ids = result.split()
            if not ids:
                return ''
            return run(['docker', 'inspect', '--format', '{{.Id}} {{.State.Running}} {{.State.StartedAt}}', *ids])

        normal_before = normal_db_state()
        process = None
        try:
            script('prepare-postgres.py')
            compose('up', '-d', '--wait', 'postgres')
            identity = json.loads(compose('ps', '--format', 'json'))
            assert identity['Project'] == project and identity['Service'] == 'postgres'
            script('bootstrap-postgres-roles.py')
            script('migrate-postgres.py', 'up')
            script('migrate-postgres.py', 'up')
            assert sql('SELECT max(version_id) FROM app.goose_db_version WHERE is_applied') == '8'
            script('verify-postgres-schema.py')
            # Extend the normal constraint suite with DDL and role-management denials.
            for role in ('watchbrief_api', 'watchbrief_user_creator', 'watchbrief_password_resetter', 'watchbrief_migrator'):
                statements = ['CREATE ROLE forbidden_verification;']
                if role != 'watchbrief_migrator':
                    statements += ['CREATE TABLE app.forbidden_verification(id int);']
                for statement in statements:
                    result = subprocess.run(['docker', 'compose', 'exec', '-T', 'postgres', 'psql', '-X', '-qAt', '-U', 'postgres', '-d', 'watchbrief', '-v', 'ON_ERROR_STOP=1', '-v', 'VERBOSITY=sqlstate'],
                                            input=f'BEGIN; SET LOCAL ROLE {role}; {statement} ROLLBACK;', text=True, capture_output=True, cwd=stage, env=environment)
                    assert result.returncode and '42501' in result.stderr, 'Privilege denial missing'
            print('Fresh bootstrap, all migrations, repeat migration and DB constraints/privileges: passed', flush=True)
            terminal = runpy.run_path(str(stage / 'scripts/verify-user-create.py'))['run_terminal']
            # The helper inherits the process environment, including the private port.
            os.environ['DB_PORT'] = environment['DB_PORT']
            passwords = ['Verification-' + secrets.token_urlsafe(24) for _ in range(3)]
            logins = ['verify_e2e_owner', 'verify_e2e_other']

            def cli(binary, login, password, success=True):
                terminal.__globals__['BINARY'] = stage / '.local/bin' / binary
                status, output = terminal(login, password, password)
                assert (status == 0) == success, 'CLI result incorrect'
                if not success:
                    assert '結果を確認できません' in output
                for forbidden in ('127.0.0.1', '5432', 'postgresql://', 'password_hash', 'SQLSTATE', *passwords):
                    assert forbidden not in output, 'Private CLI output'

            for login, password in zip(logins, passwords):
                cli('user-create', login, password)
                for seed in ('seed-collections.py', 'seed-reports.py'):
                    script(seed, '--login-id', login)
                    script(seed, '--login-id', login)
            ids = json.loads(sql("SELECT json_object_agg(u.login_id,json_build_object('collection',c.id,'report',r.id)) FROM app.users u JOIN app.generation_jobs j ON j.user_id=u.id JOIN app.collections c ON c.id=j.collection_id JOIN app.generation_attempts a ON a.generation_job_id=j.id JOIN app.reports r ON r.generation_attempt_id=a.id"))
            api_port, web_port = free_port(), free_port()
            origin = f'http://127.0.0.1:{web_port}'
            environment.update(API_ADDR=f'127.0.0.1:{api_port}', APP_ORIGIN=origin,
                               API_PASSWORD_FILE=str(stage / '.local/watchbrief-api-password'))
            logs = stage / '.local/api.log'
            with logs.open('w+b') as output:
                process = subprocess.Popen([str(stage / '.local/bin/api')], cwd=stage / 'backend', env=environment, stdout=output, stderr=output)
                for _ in range(200):
                    assert process.poll() is None, 'API startup failed'
                    if 'api_started' in logs.read_text():
                        break
                    time.sleep(.05)
                else:
                    raise AssertionError('API startup timeout')
                tokens = []
                responses = []
                auth_marker = 'Bearer ' + secrets.token_urlsafe(24)

                def request(path, token=None, data=None, supplied_origin=origin):
                    headers = {'Authorization': auth_marker}
                    if token:
                        headers['Cookie'] = 'watchbrief_session=' + token
                    if supplied_origin:
                        headers['Origin'] = supplied_origin
                    if data is not None:
                        headers['Content-Type'] = 'application/json'
                    req = urllib.request.Request(f'http://127.0.0.1:{api_port}' + path,
                                                 data=None if data is None else json.dumps(data).encode(), headers=headers)
                    try:
                        response = urllib.request.urlopen(req, timeout=20)
                    except urllib.error.HTTPError as error:
                        response = error
                    with response:
                        body = response.read()
                        if response.status >= 400:
                            responses.append(body.decode())
                        return response.status, response.headers, body

                def login(index=0, password=None):
                    status, headers, _ = request('/api/login', data={'login_id': logins[index], 'password': password or passwords[index]})
                    assert status == 204, 'Login failed'
                    cookie = SimpleCookie(headers['Set-Cookie'])
                    token = cookie['watchbrief_session'].value
                    tokens.append(token)
                    return token

                token = login()
                token2 = login()
                other_token = login(1)
                for path in ('/api/me', '/api/collections', '/api/reports'):
                    assert request(path, token)[0] == 200
                for kind in ('collections', 'reports'):
                    key = 'collection' if kind == 'collections' else 'report'
                    assert request(f'/api/{kind}/' + ids[logins[0]][key], token)[0] == 200
                    other = request(f'/api/{kind}/' + ids[logins[1]][key], token)
                    missing = request(f'/api/{kind}/00000000000000000000000000', token)
                    assert other[0] == missing[0] == 404 and other[2] == missing[2]
                for path, data in (('/api/login', {'login_id': logins[0], 'password': passwords[0]}), ('/api/logout', {})):
                    for invalid_origin in (None, 'null', 'https://external.invalid'):
                        assert request(path, token, data, invalid_origin)[0] == 403
                before = sql("SELECT json_agg(u ORDER BY id)::text FROM app.users u")
                sessions = sql("SELECT json_agg(s ORDER BY id)::text FROM app.auth_sessions s")
                compose('stop', 'postgres')
                for path, data in (('/api/login', {'login_id': logins[0], 'password': passwords[0]}),
                                   ('/api/me', None), ('/api/collections', None), ('/api/reports', None),
                                   ('/api/collections/' + ids[logins[0]]['collection'], None),
                                   ('/api/reports/' + ids[logins[0]]['report'], None), ('/api/logout', {})):
                    status, headers, body = request(path, token, data)
                    assert status == 500 and json.loads(body)['error']['code'] == 'internal_error'
                    assert headers.get('Set-Cookie') is None, 'Failure altered Cookie'
                cli('password-reset', logins[0], passwords[2], success=False)
                compose('up', '-d', '--wait', 'postgres')
                assert sql('SELECT json_agg(u ORDER BY id)::text FROM app.users u') == before
                assert sql('SELECT json_agg(s ORDER BY id)::text FROM app.auth_sessions s') == sessions
                for path in ('/api/me', '/api/collections', '/api/reports'):
                    assert request(path, token)[0] == 200
                fresh = login()
                assert request('/api/logout', fresh, {})[0] == 204
                assert request('/api/me', fresh)[0] == 401
                cli('password-reset', logins[0], passwords[2])
                for old in (token, token2):
                    assert request('/api/me', old)[0] == 401
                assert request('/api/me', other_token)[0] == 200
                assert request('/api/login', data={'login_id': logins[0], 'password': passwords[0]})[0] == 401
                token = login(password=passwords[2])
                assert request('/api/logout', token, {})[0] == 204
                assert request('/api/me', token)[0] == 401
                print('API happy path, CSRF/owner isolation, actual DB outage/recovery, reset atomicity and all old tokens rejected: passed', flush=True)
                # Browser receives private generated fixtures through a mode-600 file.
                fixture = stage / '.local/browser.json'
                fixture.write_text(json.dumps(dict(login_id=logins[0], password=passwords[2], ids=ids[logins[0]],
                                                   origin=origin, project=project, api_port=api_port)))
                print(run([node_binary, str(ROOT / 'scripts/verify-issue17-browser.mjs'), str(fixture)]), end='', flush=True)
                tokens.extend(json.loads(fixture.read_text())['browser_tokens'])
                output.flush()
                log_text = logs.read_text()
                for line in log_text.splitlines():
                    entry = json.loads(line)
                    if entry.get('level') == 'ERROR':
                        assert all(entry.get(key) for key in ('time', 'msg', 'operation', 'stage', 'result', 'version'))
                operations = {json.loads(line).get('operation') for line in log_text.splitlines() if json.loads(line).get('level') == 'ERROR'}
                assert {'login', 'current_user', 'collection_read', 'report_read', 'logout'} <= operations
                hashes = sql("SELECT string_agg(password_hash,chr(10)) FROM app.users").splitlines()
                hashes += [row['password_hash'] for row in json.loads(before)]
                token_hashes = sql("SELECT string_agg(encode(token_hash,'hex'),chr(10)) FROM app.auth_sessions").splitlines()
                token_hashes += [hashlib.sha256(base64.urlsafe_b64decode(token + '=')).hexdigest() for token in tokens]
                content = json.loads(sql("SELECT json_build_array((SELECT json_agg(research_instruction) FROM app.collections),(SELECT json_agg(body_markdown) FROM app.reports))::text"))
                private_content = [value for group in content for value in group]
                forbidden = private_content + passwords + tokens + hashes + token_hashes + [auth_marker, '127.0.0.1', 'postgresql://', 'postgres://', '$argon2id$', 'Authorization', 'Cookie', 'watchbrief_session', '架空コレクション', '外部取得を行わない検証用。', '# 架空の保存済みReport']
                forbidden += [p.read_text().strip() for p in (stage / '.local').glob('*-password')]
                for value in forbidden:
                    assert value and value not in log_text, 'Private log content'
                for body in responses:
                    assert all(value not in body for value in forbidden), 'Private API error content'
                print('Structured failure labels and API/CLI/log secret/content exclusion: passed', flush=True)
        finally:
            try:
                if process is not None and process.poll() is None:
                    process.terminate()
                    try:
                        process.wait(timeout=15)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait(timeout=5)
            finally:
                # Project name is generated here; only this run's volume is removed.
                compose('down', '--volumes', '--remove-orphans')
                assert normal_db_state() == normal_before, 'Normal DB lifecycle changed during verification'
    print('Issue #17 isolated verification passed; disposable project and private state removed.')


if __name__ == '__main__':
    main()
