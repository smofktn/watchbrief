#!/usr/bin/env python3
"""Verify owner-scoped Collection reads using fictional local fixtures only."""
from http.cookies import SimpleCookie
import json
import os
from pathlib import Path
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

ROOT = Path(__file__).resolve().parent.parent
ORIGIN = "http://127.0.0.1:5173"


def query(sql):
    result = subprocess.run(["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-qAt", "-U", "postgres", "-d", "watchbrief", "-v", "ON_ERROR_STOP=1", "-v", "VERBOSITY=sqlstate"], input=sql, cwd=ROOT, capture_output=True, text=True, timeout=15)
    return result


def fixture(name, prefix, create=False):
    path = ROOT / ".local" / name
    if create and not path.exists() and not path.is_symlink():
        with open(path, "x", opener=lambda p, flags: os.open(p, flags, 0o600)) as file:
            json.dump({"login_id": prefix + secrets.token_hex(8), "password": secrets.token_urlsafe(32)}, file)
    info = path.lstat()
    assert stat.S_ISREG(info.st_mode) and info.st_mode & 0o077 == 0, "Unsafe fixture file"
    value = json.loads(path.read_text())
    assert re.fullmatch(re.escape(prefix) + "[a-f0-9]+", value["login_id"])
    count = query(f"SELECT count(*) FROM app.users WHERE login_id='{value['login_id']}';")
    assert count.returncode == 0, "Fixture lookup failed"
    if count.stdout.strip() == "0":
        assert create, "Required fictional User missing"
        helper = runpy.run_path(str(ROOT / "scripts/verify-user-create.py"))
        code, _ = helper["run_terminal"](value["login_id"], value["password"], value["password"])
        assert code == 0, "Fictional User creation failed"
    return value


def main():
    primary = fixture("user-create-verification.json", "verify_cli_")
    other = fixture("logout-verification.json", "verify_logout_")
    empty = fixture("collections-empty-verification.json", "verify_empty_", create=True)
    seed = runpy.run_path(str(ROOT / "scripts/seed-collections.py"))["seed"]
    ids = seed(primary["login_id"])
    other_ids = seed(other["login_id"])
    # Repeating seed must not add rows, edit settings, or change their timestamps.
    before = query("SELECT row_to_json(c)::text FROM app.collections c ORDER BY id;")
    assert before.returncode == 0
    seed(primary["login_id"])
    after = query("SELECT row_to_json(c)::text FROM app.collections c ORDER BY id;")
    assert after.returncode == 0 and before.stdout == after.stdout, "Seed is not idempotent"
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
            for path in ("/api/collections", "/api/collections/" + ids["collection"]):
                for invalid in (None, "invalid", token + "; watchbrief_session=invalid"):
                    status, headers, body = request(path, invalid)
                    assert status == 401 and json.loads(body)["error"]["code"] == "unauthenticated", "Unauthenticated Collection exposed"
                    assert headers.get("Cache-Control") == "no-store"
            status, headers, body = request("/api/collections", token)
            rows = json.loads(body)["collections"]
            assert status == 200 and headers.get("Cache-Control") == "no-store"
            assert any(row["id"] == ids["collection"] for row in rows)
            assert not any(row["id"] == other_ids["collection"] for row in rows), "Other User leaked in list"
            status, _, body = request("/api/collections/" + ids["collection"], token)
            value = json.loads(body)
            assert status == 200 and value["id"] == ids["collection"] and "user_id" not in value
            assert value["schedule"]["cron_expression"] == "0 7 * * *"
            assert value["targets"][0]["sources"][0]["url"] == "https://example.com/"
            denied = request("/api/collections/" + other_ids["collection"], token)
            missing = request("/api/collections/00000000000000000000000000", token)
            assert denied[0] == missing[0] == 404 and denied[2] == missing[2], "Other ownership revealed"
            assert request("/api/collections/" + ids["collection"], other_token)[0] == 404
            assert request("/api/collections/invalid", token)[0] == 400
            assert request("/api/collections/invalid")[0] == 401
            status, _, body = request("/api/collections", empty_token)
            assert status == 200 and json.loads(body) == {"collections": []}, "Empty Collection list is not []"
            for row in rows:
                assert set(row) == {"id", "name", "enabled", "display_order"}
            print("Collection seed repeat, own list/detail, nested settings, empty arrays, 401/400 and indistinguishable owner/missing 404: passed")

            tables = ["collections", "collection_schedules", "watch_targets", "information_sources"]
            for table in tables:
                allowed = query(f"BEGIN; SET LOCAL ROLE watchbrief_api; SELECT 1 FROM app.{table} LIMIT 0; ROLLBACK;")
                assert allowed.returncode == 0, "API cannot read settings"
                denied_write = query(f"BEGIN; SET LOCAL ROLE watchbrief_api; DELETE FROM app.{table} WHERE false; ROLLBACK;")
                assert denied_write.returncode != 0 and "42501" in denied_write.stderr, "API got settings write privilege"
                denied_creator = query(f"BEGIN; SET LOCAL ROLE watchbrief_user_creator; SELECT 1 FROM app.{table} LIMIT 0; ROLLBACK;")
                assert denied_creator.returncode != 0 and "42501" in denied_creator.stderr, "User creator got settings privilege"
            owner = query(f"SELECT user_id FROM app.collections WHERE id='{ids['collection']}';").stdout.strip()
            base = "BEGIN; SET LOCAL ROLE watchbrief_migrator; "
            tests = [
                (f"INSERT INTO app.collections(id,user_id,name,initial_lookback_mode,initial_lookback_days) VALUES('00000000000000000000000001','{owner}','x','days',NULL);", "23514"),
                (f"INSERT INTO app.collections(id,user_id,name,initial_lookback_mode,initial_lookback_days) VALUES('00000000000000000000000001','{owner}','x','days',366);", "23514"),
                (f"INSERT INTO app.collections(id,user_id,name,initial_lookback_days) VALUES('00000000000000000000000001','{owner}','x',1);", "23514"),
                ("INSERT INTO app.collections(id,user_id,name) VALUES('00000000000000000000000001','00000000000000000000000000','x');", "23503"),
                (f"INSERT INTO app.collection_schedules(id,collection_id,cron_expression) VALUES('00000000000000000000000001','{ids['collection']}','0 8 * * *');", "23505"),
                (f"INSERT INTO app.watch_targets(id,collection_id,name) VALUES('invalid','{ids['collection']}','x');", "23514"),
            ]
            for statement, code in tests:
                result = query(base + statement + "ROLLBACK;")
                assert result.returncode != 0 and code in result.stderr, "Collection constraint missing"
            # Cascade only fictional data in a rolled-back transaction; no history exists yet.
            cascade = query(base + f"DELETE FROM app.collections WHERE id='{ids['collection']}'; SELECT count(*) FROM app.watch_targets WHERE id='{ids['target']}'; ROLLBACK;")
            assert cascade.returncode == 0 and cascade.stdout.strip() == "0", "Current-setting cascade failed"
            output = os.pread(logs.fileno(), 1048576, 0).decode()
            for secret in [primary["password"], other["password"], empty["password"], *issued]:
                assert secret not in output, "Private value in API logs"
            assert value["research_instruction"] not in output, "Settings text in API logs"
            print("Settings SELECT-only grants, creator isolation, lookback/FK/unique/ULID constraints, rolled-back cascade and log privacy: passed")
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
