#!/usr/bin/env python3
"""Verify the dedicated reset fixture via hidden TTY and local HTTP; never print secrets."""
import hashlib
import base64
from http.cookies import SimpleCookie
import json
import os
import re
import runpy
import secrets
import socket
import stat
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ORIGIN = "http://127.0.0.1:5173"
COOKIE = "watchbrief_session"
helper = runpy.run_path(str(ROOT / "scripts/verify-user-create.py"))
query = runpy.run_path(str(ROOT / "scripts/verify-logout-api.py"))["query"]


def terminal(binary, login_id, first, second, interrupt=False):
    helper["run_terminal"].__globals__["BINARY"] = ROOT / ".local/bin" / binary
    return helper["run_terminal"](login_id, first, second, interrupt)


def main():
    path = ROOT / ".local/password-reset-verification.json"
    if not path.exists() and not path.is_symlink():
        with open(path, "x", opener=lambda p, flags: os.open(p, flags, 0o600)) as file:
            json.dump({"login_id": "verify_reset_" + secrets.token_hex(8),
                       "password": secrets.token_urlsafe(32), "new_password": secrets.token_urlsafe(32)}, file)
    info = path.lstat()
    assert stat.S_ISREG(info.st_mode) and info.st_mode & 0o077 == 0, "Unsafe fixture file"
    fixture = json.loads(path.read_text())
    login_id, old, new = fixture["login_id"], fixture["password"], fixture["new_password"]
    assert re.fullmatch(r"verify_reset_[a-f0-9]{16}", login_id)
    if query(f"SELECT count(*) FROM app.users WHERE login_id='{login_id}';") == "0":
        assert terminal("user-create", login_id, old, old)[0] == 0
    assert terminal("password-reset", login_id, old, old)[0] == 0
    user_id = query(f"SELECT id FROM app.users WHERE login_id='{login_id}';")
    assert re.fullmatch(r"[0-7][0-9A-HJKMNP-TV-Z]{25}", user_id)
    # Give this fixture settings/history as well as comparing all existing business rows.
    seed = subprocess.run(["python3", "scripts/seed-reports.py", "--login-id", login_id], cwd=ROOT, capture_output=True)
    assert seed.returncode == 0, "Reset fixture seed failed"

    tables = query("SELECT tablename FROM pg_tables WHERE schemaname='app' ORDER BY tablename;").splitlines()
    def snapshot():
        result = {}
        for table in tables:
            assert re.fullmatch(r"[a-z_]+", table)
            condition = f" WHERE id <> '{user_id}'" if table == "users" else (
                f" WHERE user_id <> '{user_id}'" if table == "auth_sessions" else "")
            result[table] = hashlib.sha256(query(f"SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]')::text FROM (SELECT * FROM app.{table}{condition}) t;").encode()).hexdigest()
        return result
    before = snapshot()
    def saved():
        return query(f"SELECT json_build_array(password_hash, updated_at)::text FROM app.users WHERE id='{user_id}';")
    unchanged = saved()
    for first, second, interrupt in [(old,new,False),("short","short",False),("","",True)]:
        assert terminal("password-reset", login_id, first, second, interrupt)[0] != 0
        assert saved() == unchanged, "Failed input changed password"
    assert terminal("password-reset", login_id + "_missing", old, old)[0] != 0
    no_tty = subprocess.run([str(ROOT / ".local/bin/password-reset"), "--login-id", login_id], cwd=ROOT / "backend", start_new_session=True, stdin=subprocess.DEVNULL, capture_output=True)
    assert no_tty.returncode != 0 and saved() == unchanged
    print("Reset TTY: mismatch/short/missing User/no terminal/Ctrl+C rejected; echo restored; secrets absent")

    with socket.socket() as reserved:
        reserved.bind(("127.0.0.1", 0))
        port = reserved.getsockname()[1]
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    issued = []
    def request(endpoint, token=None, password=None):
        headers = {"Origin": ORIGIN}
        if token:
            headers["Cookie"] = COOKIE + "=" + token
        body = None
        if password:
            headers["Content-Type"] = "application/json"
            body = json.dumps({"login_id": login_id,"password": password}).encode()
        req = urllib.request.Request(f"http://127.0.0.1:{port}" + endpoint, headers=headers, data=body)
        try:
            response = client.open(req, timeout=15)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            return response.status, response.headers, response.read()
    def login(password):
        status,headers,body = request("/api/login",password=password)
        assert status == 204 and not body, "Expected login success"
        cookie = SimpleCookie(); cookie.load(headers["Set-Cookie"])
        token = cookie[COOKIE].value; issued.append(token)
        return token
    environment = {**os.environ,"API_ADDR":f"127.0.0.1:{port}","APP_ORIGIN":ORIGIN,"API_PASSWORD_FILE":str(ROOT / ".local/watchbrief-api-password")}
    with tempfile.TemporaryFile(mode="w+b", dir=ROOT / ".local") as logs:
        process = subprocess.Popen([str(ROOT / ".local/bin/api")],cwd=ROOT / "backend",env=environment,stdout=logs,stderr=logs)
        try:
            deadline = time.monotonic()+10
            while b'"msg":"api_started"' not in os.pread(logs.fileno(),65536,0):
                assert process.poll() is None and time.monotonic()<deadline,"API startup failed"
                time.sleep(.05)
            tokens = [login(old), login(old)]
            assert all(request("/api/me", token)[0] == 200 for token in tokens)
            assert terminal("password-reset",login_id,new,new)[0] == 0
            assert all(request("/api/me", token)[0] == 401 for token in tokens), "Old token accepted"
            status,headers,_ = request("/api/login",password=old)
            assert status == 401 and not headers.get("Set-Cookie"), "Old password accepted"
            token = login(new)
            assert request("/api/me",token)[0] == 200
            previous = saved()
            assert terminal("password-reset",login_id,new,new)[0] == 0
            assert saved() != previous and request("/api/me",token)[0] == 401,"Same-password reset reused hash/session"
            login(new)
            assert snapshot() == before,"Other User/sessions/settings/history changed"
            log = os.pread(logs.fileno(),1024*1024,0).decode()
            values = [old,new,*issued,query(f"SELECT password_hash FROM app.users WHERE id='{user_id}';")]
            values += [hashlib.sha256(base64.urlsafe_b64decode(value + "=")).hexdigest() for value in issued]
            values += [(ROOT / ".local" / filename).read_text().rstrip("\n") for filename in
                       ("watchbrief-api-password", "watchbrief-password-resetter-password", "watchbrief-user-creator-password", "watchbrief-migrator-password")]
            for value in values:
                assert value not in log,"Private value appeared in API logs"
            print("Two old tokens 401; old password 401; new password succeeds; same-password reset invalidates sessions")
            print("All other Users/sessions and every business row preserved; API logs contain no supplied secrets")
        finally:
            process.terminate();process.wait(timeout=15)
    assert terminal("password-reset",login_id,new,new)[0] == 0
    assert snapshot() == before
    print("Dedicated reset fixture retained; its test sessions removed; existing fixture passwords preserved")


if __name__ == "__main__":
    main()
