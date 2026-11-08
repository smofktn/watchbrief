#!/usr/bin/env python3
"""Verify local login with the private fictional CLI fixture; never print credentials/Cookies."""
import base64
import hashlib
import http.cookiejar
from http.cookies import SimpleCookie
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import time
import unicodedata
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent


def query(statement):
    result = subprocess.run(["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-U", "postgres",
                             "-d", "watchbrief", "-v", "ON_ERROR_STOP=1", "-Atc", statement],
                            cwd=ROOT, capture_output=True, text=True)
    if result.returncode:
        raise SystemExit("DB verification failed; no details printed.")
    return result.stdout.strip()


def main():
    path = ROOT / ".local/user-create-verification.json"
    if not path.is_file():
        raise SystemExit("Run make verify-user-create first.")
    assert not path.is_symlink() and path.stat().st_mode & 0o077 == 0
    credentials = json.loads(path.read_text())
    login, password = credentials["login_id"], credentials["password"]
    assert re.fullmatch(r"verify_cli_[a-f0-9]+", login)
    assert query(f"SELECT count(*) FROM app.users WHERE login_id='{login}';") == "1"
    with socket.socket() as reserved:
        reserved.bind(("127.0.0.1", 0))
        port = reserved.getsockname()[1]
    origin = "http://127.0.0.1:5173"
    url = f"http://127.0.0.1:{port}/api/login"
    environment = {**os.environ, "API_ADDR": f"127.0.0.1:{port}", "APP_ORIGIN": origin,
                   "API_PASSWORD_FILE": str(ROOT / ".local/watchbrief-api-password")}
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

    def request(login_id, supplied, request_origin=origin):
        headers = {"Content-Type": "application/json"}
        if request_origin:
            headers["Origin"] = request_origin
        body = json.dumps({"login_id": login_id, "password": supplied}).encode()
        req = urllib.request.Request(url, data=body, headers=headers)
        try:
            response = client.open(req, timeout=15)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            return response.status, response.headers, response.read()

    def sessions():
        text = query(f"SELECT coalesce(json_agg(json_build_object('id',s.id,'hash',encode(s.token_hash,'hex'))),'[]'::json)::text FROM app.auth_sessions s JOIN app.users u ON u.id=s.user_id WHERE u.login_id='{login}';")
        return json.loads(text)

    with tempfile.TemporaryFile(mode="w+b", dir=ROOT / ".local") as logs:
        process = subprocess.Popen([str(ROOT / ".local/bin/api")], cwd=ROOT / "backend", env=environment,
                                   stdout=logs, stderr=logs)

        def current_user(cookie_header=None, use_jar=True):
            headers = {} if cookie_header is None else {"Cookie": cookie_header}
            req = urllib.request.Request(url.replace("/api/login", "/api/me"), headers=headers)
            opener = client if use_jar else urllib.request.build_opener()
            try:
                response = opener.open(req, timeout=15)
            except urllib.error.HTTPError as exc:
                response = exc
            with response:
                return response.status, response.headers, response.read()

        def verify_current_user(expected_token):
            retained = sessions()
            status, headers, body = current_user()
            assert status == 200 and json.loads(body) == {"login_id": login}
            assert headers.get("Cache-Control") == "no-store"
            cookie = SimpleCookie()
            cookie.load(headers["Set-Cookie"])
            value = cookie["watchbrief_session"]
            assert value.value == expected_token and value["max-age"] == "34560000"
            assert value["httponly"] and value["samesite"].lower() == "lax"
            assert value["path"] == "/" and not value["domain"] and not value["secure"]
            assert sessions() == retained

        def read_logs(offset=0):
            return os.pread(logs.fileno(), 1024 * 1024, offset)

        def wait_ready(offset=0):
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                assert process.poll() is None, "API exited during startup"
                if b'"msg":"api_started"' in read_logs(offset):
                    return
                time.sleep(0.05)
            raise AssertionError("API readiness timed out")

        try:
            wait_ready()
            before = sessions()
            unknown_token = base64.urlsafe_b64encode(os.urandom(32)).decode().rstrip("=")
            for invalid in (None, "watchbrief_session=bad", "watchbrief_session=" + unknown_token,
                            "watchbrief_session=" + unknown_token + "; watchbrief_session=bad"):
                status, headers, body = current_user(invalid, use_jar=False)
                assert status == 401 and json.loads(body)["error"]["code"] == "unauthenticated"
                assert headers.get("Set-Cookie") is None and headers.get("Cache-Control") == "no-store"
            assert sessions() == before
            failures = []
            for attempted_id, supplied in [(login, password + "Z"), (login + "_missing", password)]:
                status, headers, body = request(attempted_id, supplied)
                assert status == 401 and headers.get("Set-Cookie") is None
                assert json.loads(body)["error"]["code"] == "invalid_credentials"
                failures.append(body)
            assert failures[0] == failures[1] and sessions() == before
            for supplied_origin in (None, "null", "http://external.invalid"):
                status, headers, _ = request(login, password, supplied_origin)
                assert status == 403 and headers.get("Set-Cookie") is None
            assert sessions() == before
            print("Unknown ID/wrong password share 401; CSRF rejects before session creation")
            issued = []
            for _ in range(2):
                status, headers, body = request(login.upper(), unicodedata.normalize("NFD", password))
                assert status == 204 and body == b"" and headers.get("Cache-Control") == "no-store"
                cookie = SimpleCookie()
                cookie.load(headers["Set-Cookie"])
                value = cookie["watchbrief_session"]
                assert value["httponly"] and value["samesite"].lower() == "lax" and value["path"] == "/"
                assert not value["secure"] and not value["domain"] and value["max-age"] == "34560000"
                raw = base64.urlsafe_b64decode(value.value + "=")
                assert len(raw) == 32 and len(value.value) == 43
                digest = hashlib.sha256(raw).hexdigest()
                after = sessions()
                matches = [row for row in after if row["hash"] == digest]
                assert len(matches) == 1 and matches[0]["id"] != value.value
                issued.append(value.value)
                verify_current_user(value.value)
            assert issued[0] != issued[1] and len(sessions()) == len(before) + 2
            print("NFC login 204, Cookie attributes, fresh random tokens and SHA-256-only DB storage: passed")
            retained = sessions()
            process.terminate()
            assert process.wait(timeout=15) == 0
            offset = os.fstat(logs.fileno()).st_size
            process = subprocess.Popen([str(ROOT / ".local/bin/api")], cwd=ROOT / "backend", env=environment,
                                       stdout=logs, stderr=logs)
            wait_ready(offset)
            assert sessions() == retained
            verify_current_user(issued[-1])
            print("Cookie authenticates /api/me before and after API restart; same token renewed, DB sessions unchanged")
            output = read_logs().decode()
            assert password not in output and all(value not in output for value in issued)
            assert all(row["hash"] not in output for row in retained)
            print("API log inspection: no submitted password or issued token; test sessions retained")
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=15)


if __name__ == "__main__":
    main()
