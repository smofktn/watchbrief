#!/usr/bin/env python3
"""Verify logout against local HTTP/PostgreSQL without printing private credentials or tokens."""
import base64
import hashlib
from http.cookies import SimpleCookie
import json
import os
from pathlib import Path
import re
import runpy
import secrets
import select
import socket
import stat
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
ORIGIN = "http://127.0.0.1:5173"
COOKIE = "watchbrief_session"


def query(statement):
    result = subprocess.run(
        ["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-qAt", "-U", "postgres",
         "-d", "watchbrief", "-v", "ON_ERROR_STOP=1"],
        input=statement, cwd=ROOT, capture_output=True, text=True, timeout=15,
    )
    if result.returncode:
        raise SystemExit("DB verification failed; private details suppressed.")
    return result.stdout.strip()


def credentials(path):
    info = path.lstat()
    assert stat.S_ISREG(info.st_mode) and info.st_mode & 0o077 == 0, "Unsafe fixture file"
    value = json.loads(path.read_text())
    assert re.fullmatch(r"verify_(?:cli|logout)_[a-f0-9]+", value["login_id"])
    assert isinstance(value["password"], str)
    return value


def main():
    primary_path = ROOT / ".local/user-create-verification.json"
    if not primary_path.exists():
        raise SystemExit("Run make verify-user-create first.")
    primary = credentials(primary_path)
    secondary_path = ROOT / ".local/logout-verification.json"
    if not secondary_path.exists() and not secondary_path.is_symlink():
        with open(secondary_path, "x", opener=lambda p, flags: os.open(p, flags, 0o600)) as file:
            json.dump({"login_id": "verify_logout_" + secrets.token_hex(8),
                       "password": secrets.token_urlsafe(32)}, file)
    secondary = credentials(secondary_path)
    assert primary["login_id"] != secondary["login_id"]
    if query(f"SELECT count(*) FROM app.users WHERE login_id='{secondary['login_id']}';") == "0":
        # Reuse the existing non-echoing CLI harness; only this dedicated fixture is created.
        helper = runpy.run_path(str(ROOT / "scripts/verify-user-create.py"))
        code, _ = helper["run_terminal"](secondary["login_id"], secondary["password"], secondary["password"])
        assert code == 0, "Additional fictional User creation failed"

    def sessions():
        return json.loads(query("SELECT coalesce(json_agg(json_build_object('id',id,'user',user_id,'hash',encode(token_hash,'hex')) ORDER BY id),'[]'::json)::text FROM app.auth_sessions;"))

    baseline = sessions()
    with socket.socket() as reserved:
        reserved.bind(("127.0.0.1", 0))
        port = reserved.getsockname()[1]
    url = f"http://127.0.0.1:{port}"
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def request(path, token=None, method="GET", origin=ORIGIN, body=None):
        headers = {"Origin": origin} if origin is not None else {}
        if token is not None:
            headers["Cookie"] = COOKIE + "=" + token
        if body is not None:
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(url + path, method=method, headers=headers, data=body)
        try:
            response = client.open(req, timeout=15)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            return response.status, response.headers, response.read()

    issued = []
    digests = []

    def login(fixture):
        status, headers, body = request("/api/login", method="POST", body=json.dumps(fixture).encode())
        assert status == 204 and body == b"", "Login failed"
        cookie = SimpleCookie()
        cookie.load(headers["Set-Cookie"])
        token = cookie[COOKIE].value
        assert len(token) == 43
        issued.append(token)
        digests.append(hashlib.sha256(base64.urlsafe_b64decode(token + "=")).hexdigest())
        return token

    def assert_me(token, expected):
        status, _, body = request("/api/me", token)
        if expected is None:
            assert status == 401 and json.loads(body)["error"]["code"] == "unauthenticated", "Old token still authenticates"
        else:
            assert status == 200 and json.loads(body) == {"login_id": expected}, "Other session lost authentication"

    def assert_logout(token):
        status, headers, body = request("/api/logout", token, method="POST")
        assert status == 204 and body == b"" and headers.get("Cache-Control") == "no-store"
        cookie = SimpleCookie()
        cookie.load(headers["Set-Cookie"])
        deleted = cookie[COOKIE]
        assert deleted.value == "" and deleted["max-age"] == "0" and deleted["expires"]
        assert deleted["path"] == "/" and not deleted["domain"]
        assert deleted["httponly"] and deleted["samesite"].lower() == "lax" and not deleted["secure"]

    environment = {**os.environ, "API_ADDR": f"127.0.0.1:{port}", "APP_ORIGIN": ORIGIN,
                   "API_PASSWORD_FILE": str(ROOT / ".local/watchbrief-api-password")}
    with tempfile.TemporaryFile(mode="w+b", dir=ROOT / ".local") as logs:
        process = subprocess.Popen([str(ROOT / ".local/bin/api")], cwd=ROOT / "backend", env=environment,
                                   stdout=logs, stderr=logs)
        try:
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                assert process.poll() is None, "API startup failed"
                if b'"msg":"api_started"' in os.pread(logs.fileno(), 65536, 0):
                    break
                time.sleep(0.05)
            else:
                raise AssertionError("API readiness timed out")
            target, same_user, other_user = login(primary), login(primary), login(secondary)
            before = sessions()
            assert len(before) == len(baseline) + 3
            assert_me(target, primary["login_id"])
            for bad_origin in (None, "null", "https://external.invalid", "http://127.0.0.1:5174"):
                status, headers, body = request("/api/logout", target, "POST", bad_origin)
                assert status == 403 and headers.get("Set-Cookie") is None
                assert json.loads(body)["error"]["code"] == "csrf_rejected"
            for invalid in (None, "bad", target + "; " + COOKIE + "=bad"):
                status, headers, body = request("/api/logout", invalid, "POST")
                assert status == 401 and headers.get("Set-Cookie") is None
                assert json.loads(body)["error"]["code"] == "unauthenticated"
            status, headers, _ = request("/api/logout", target)
            assert status == 405 and headers.get("Set-Cookie") is None and sessions() == before
            print("GET/invalid CSRF/invalid or duplicate Cookie rejected without session changes")
            assert_logout(target)
            assert_me(target, None)
            assert_logout(target)
            assert_me(same_user, primary["login_id"])
            assert_me(other_user, secondary["login_id"])
            assert sessions() == [row for row in before if row["hash"] != digests[0]]
            print("Login -> logout -> saved token 401; repeated logout 204; other sessions preserved")

            # Lock only the test session, leaving existing sessions and reads usable.
            locker = subprocess.Popen(
                ["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-qAt", "-U", "postgres",
                 "-d", "watchbrief", "-v", "ON_ERROR_STOP=1"],
                cwd=ROOT, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            )
            try:
                locker.stdin.write(("BEGIN; SET LOCAL idle_in_transaction_session_timeout='30s'; "
                                    f"SELECT 1 FROM app.auth_sessions WHERE token_hash=decode('{digests[1]}','hex') FOR UPDATE;\n"
                                    "\\echo logout_lock_ready\n").encode())
                locker.stdin.flush()
                output = b""
                deadline = time.monotonic() + 10
                while b"logout_lock_ready" not in output:
                    assert locker.poll() is None and time.monotonic() < deadline, "Could not lock test row"
                    if select.select([locker.stdout], [], [], 0.1)[0]:
                        output += os.read(locker.stdout.fileno(), 4096)
                before_failure = sessions()
                status, headers, body = request("/api/logout", same_user, "POST")
                assert status == 500 and json.loads(body)["error"]["code"] == "internal_error"
                assert headers.get("Set-Cookie") is None and sessions() == before_failure
                assert_me(same_user, primary["login_id"])
                assert_me(other_user, secondary["login_id"])
            finally:
                try:
                    locker.communicate(input=b"ROLLBACK;\n", timeout=5)
                except subprocess.TimeoutExpired:
                    locker.kill()
                    locker.communicate()
                assert locker.returncode == 0, "Test row lock release failed"
            assert_logout(same_user)
            assert_me(same_user, None)
            assert_logout(other_user)
            assert sessions() == baseline, "Existing sessions changed or test sessions remain"
            print("Real DB timeout: 500 without Cookie deletion; session preserved; retry succeeds after lock release")
            log = os.pread(logs.fileno(), 1024 * 1024, 0).decode()
            assert '"msg":"logout_failed"' in log and '"stage":"session_delete"' in log
            for private in [primary["password"], secondary["password"], *issued, *digests]:
                assert private not in log, "Private value appeared in API log"
            print("API failure log sanitized; all sessions issued by this run removed; existing sessions unchanged")
        finally:
            try:
                if process.poll() is None:
                    cleanup_failed = False
                    for token in issued:
                        try:
                            assert_logout(token)
                        except (AssertionError, OSError):
                            # Try all issued tokens even if one cleanup request fails.
                            cleanup_failed = True
                    assert not cleanup_failed, "Test-session cleanup incomplete; existing sessions were not targeted"
            finally:
                if process.poll() is None:
                    process.terminate()
                    process.wait(timeout=15)


if __name__ == "__main__":
    main()
