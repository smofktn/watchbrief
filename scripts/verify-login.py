#!/usr/bin/env python3
"""Exercise real command + HTTP + API process restart using fictional data only.
Requires migrated local DB, built bin/api and bin/user-create, private DB JSON files.
Never prints credentials, cookies, database errors or captured application output.
"""
import argparse
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import pty
import secrets
import select
import signal
import subprocess
import termios
import time
import urllib.error
import urllib.request

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--api-config', type=Path, required=True)
parser.add_argument('--admin-config', type=Path, required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
origin = 'http://127.0.0.1:5173'
base = 'http://127.0.0.1:8080'
login = 'fictional_' + secrets.token_hex(6)
password = secrets.token_urlsafe(32)


def require(ok, label):
    if not ok:
        raise RuntimeError(label)


def create_user(duplicate=False):
    master, slave = pty.openpty()
    def terminal():
        os.setsid()
        import fcntl
        fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
    env = {**os.environ, 'DB_CONFIG_FILE': str(args.admin_config.resolve())}
    proc = subprocess.Popen([str(root/'bin/user-create'), '--login-id', login.upper()],
                            stdin=slave, stdout=slave, stderr=slave, env=env, preexec_fn=terminal)
    output = b''
    try:
        # Wait for ReadPassword to disable echo before each secret write.
        for prompt in (b'Password (', b'Confirm password:'):
            end = time.monotonic() + 15
            while prompt not in output or termios.tcgetattr(slave)[3] & termios.ECHO:
                require(time.monotonic() < end and proc.poll() is None, 'hidden terminal prompt unavailable')
                if select.select([master], [], [], .05)[0]:
                    output += os.read(master, 8192)
            os.write(master, password.encode() + b'\n')
        end = time.monotonic() + 15
        while proc.poll() is None:
            require(time.monotonic() < end, 'user-create timeout')
            if select.select([master], [], [], .05)[0]:
                output += os.read(master, 8192)
        while select.select([master], [], [], 0)[0]:
            output += os.read(master, 8192)
        require(password.encode() not in output, 'password echoed by administrative command')
        require(proc.returncode == (1 if duplicate else 0), 'user-create unexpected result')
        require((b'login ID already exists' if duplicate else b'User created.') in output, 'user-create result missing')
    finally:
        if proc.poll() is None:
            proc.kill(); proc.wait()
        os.close(master); os.close(slave)


jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar))


def request(method, path, body=None, headers=None, with_cookies=True):
    req = urllib.request.Request(base + path, data=None if body is None else json.dumps(body).encode(),
                                 method=method, headers=headers or {})
    client = opener if with_cookies else urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        response = client.open(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, response.read(), response.headers


def start_api(log):
    env = {**os.environ, 'DB_CONFIG_FILE': str(args.api_config.resolve()),
           'APP_ORIGIN': origin, 'COOKIE_MODE': 'local-http'}
    proc = subprocess.Popen([str(root/'bin/api')], env=env, stdout=log, stderr=log)
    end = time.monotonic() + 15
    while time.monotonic() < end:
        require(proc.poll() is None, 'API exited before ready')
        try:
            if request('GET', '/api/me', with_cookies=False)[0] == 401:
                return proc
        except urllib.error.URLError:
            time.sleep(.05)
    proc.terminate(); proc.wait(timeout=15)
    raise RuntimeError('API startup timeout')


def stop_api(proc):
    proc.send_signal(signal.SIGTERM)
    proc.wait(timeout=15)


def verify_api_db_permissions():
    # Authenticate as the actual API role; SET ROLE under a bootstrap session alone
    # would not prove that a login role cannot switch to the schema owner.
    cfg = json.loads(args.api_config.read_text())
    require(cfg['host'] == '127.0.0.1' and cfg['user'] == 'watchbrief_api', 'unexpected test DB target')
    env = {**os.environ, 'PGPASSWORD':cfg['password'], 'PGSSLMODE':'disable'}
    statements = [
        'CREATE TABLE forbidden_probe(id int)',
        'ALTER TABLE users ADD COLUMN forbidden_probe int',
        'CREATE ROLE forbidden_probe',
        'SET ROLE watchbrief_owner',
        'CREATE SCHEMA forbidden_probe',
        'CREATE TEMP TABLE forbidden_probe(id int)',
    ]
    for statement in statements:
        result = subprocess.run(['psql', '-X', '--no-password', '-h', cfg['host'], '-p', str(cfg['port']),
                                 '-U', cfg['user'], '-d', cfg['database'], '-v', 'ON_ERROR_STOP=1',
                                 '-v', 'VERBOSITY=sqlstate'],
                                input=('BEGIN; ' + statement + '; ROLLBACK;').encode(),
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, timeout=15)
        require(result.returncode == 3 and b'42501' in result.stderr, 'API DB permission probe did not reject with insufficient_privilege')


def main():
    # Refuse to use an existing listener; the script owns exactly its API process.
    import socket
    with socket.socket() as probe:
        require(probe.connect_ex(('127.0.0.1',8080)) != 0, 'port 8080 is already occupied')
    verify_api_db_permissions()
    create_user()
    create_user(duplicate=True)
    logdir = Path('/tmp/watchbrief-supervision/issue-53')
    logdir.mkdir(mode=0o700, parents=True, exist_ok=True)
    import tempfile
    with tempfile.TemporaryFile(dir=logdir) as log:
        proc = start_api(log)
        try:
            hs = {'Origin':origin, 'X-WatchBrief-CSRF':'1', 'Content-Type':'application/json'}
            payload = {'login_id':login.upper(), 'password':password}
            require(request('GET','/api/me',with_cookies=False)[0] == 401,'unauthenticated accepted')
            require(request('POST','/api/login',payload,{'Content-Type':'application/json'})[0] == 403,'missing CSRF accepted')
            require(request('POST','/api/login',payload,{**hs,'Origin':'https://evil.invalid'})[0] == 403,'cross-origin accepted')
            require(request('POST','/api/login',{**payload,'password':secrets.token_urlsafe(32)},hs)[0] == 401,'wrong password accepted')
            require(request('POST','/api/login',{**payload,'login_id':'invalid@id'},hs)[0] == 400,'bad login ID accepted')
            require(request('POST','/api/login',payload,hs)[0] == 200,'login failed')
            cookies = list(jar)
            require(len(cookies)==1,'missing session cookie')
            cookie = cookies[0]
            require(cookie.expires is not None and cookie.has_nonstandard_attr('HttpOnly') and not cookie.secure,'local persistent cookie attributes')
            status, body, _ = request('GET','/api/me')
            require(status==200 and json.loads(body)['login_id']==login,'identity mismatch')
            require(password.encode() not in body and cookie.value.encode() not in body and b'password_hash' not in body,'response secret leak')
            stop_api(proc)
            proc = start_api(log)
            require(request('GET','/api/me')[0]==200,'session lost after actual API restart')
            # Stop before reading to avoid racing the child's shared file offset.
            stop_api(proc)
            # Disk-free browser-like cookie jar is retained across the API process restart.
            log.seek(0)
            output = log.read()
            digest = hashlib.sha256(bytes.fromhex(cookie.value)).hexdigest()
            for value in (password, cookie.value, digest, '$argon2id$', login):
                require(value.encode() not in output,'application log secret/input leak')
            print('PASS: hidden User creation/duplicate, login/me, invalid credentials/input, CSRF, persistent cookie, API restart, actual API DB privilege rejection, log/response exclusion.')
        finally:
            if proc.poll() is None:
                stop_api(proc)
    print('Fictional User and sessions remain in the dedicated local test DB; no secret was printed.')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Print only our fixed labels, never library exceptions or captured output.
        label = str(error) if type(error) is RuntimeError else type(error).__name__
        raise SystemExit('FAIL: ' + label) from None
