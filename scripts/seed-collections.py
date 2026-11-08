#!/usr/bin/env python3
"""Add one fictional, stopped Collection to an existing User in the local DB."""
import argparse
import hashlib
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parent.parent


def query(sql, login):
    result = subprocess.run(
        ["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-qAt", "-U", "postgres",
         "-d", "watchbrief", "-v", "ON_ERROR_STOP=1", "-v", "fixture_login=" + login],
        input=sql, cwd=ROOT, capture_output=True, text=True, timeout=20,
    )
    if result.returncode:
        raise SystemExit("Fictional Collection setup failed; existing settings were not overwritten. Private DB details suppressed.")
    return result.stdout.strip()


def fixture_id(owner, kind):
    return "0" + hashlib.sha256(("watchbrief-collection-fixture-v1:" + owner + ":" + kind).encode()).hexdigest()[:25].upper()


def seed(login):
    owner = query("SELECT id FROM app.users WHERE login_id=:'fixture_login';", login)
    if not re.fullmatch(r"[0-7][0-9A-HJKMNP-TV-Z]{25}", owner):
        raise SystemExit("Create the fictional User with make user-create first.")
    ids = {kind: fixture_id(owner, kind) for kind in ("collection", "schedule", "target", "source")}
    # Repeating the command never edits existing data. Any conflicting fixture content
    # aborts the entire transaction rather than converting unrelated rows into fixtures.
    sql = f"""BEGIN;
SET LOCAL ROLE watchbrief_migrator;
INSERT INTO app.collections(id,user_id,name,research_instruction,enabled,display_order)
VALUES ('{ids['collection']}','{owner}','架空コレクション','架空データで設定の取得を確認する。',false,0) ON CONFLICT(id) DO NOTHING;
INSERT INTO app.collection_schedules(id,collection_id,cron_expression)
VALUES ('{ids['schedule']}','{ids['collection']}','0 7 * * *') ON CONFLICT(id) DO NOTHING;
INSERT INTO app.watch_targets(id,collection_id,name,instruction,enabled,display_order)
VALUES ('{ids['target']}','{ids['collection']}','架空の調査対象','検証専用。外部へアクセスしない。',true,0) ON CONFLICT(id) DO NOTHING;
INSERT INTO app.information_sources(id,watch_target_id,name,url,instruction)
VALUES ('{ids['source']}','{ids['target']}','架空の情報源','https://example.com/','取得処理は実行しない。') ON CONFLICT(id) DO NOTHING;
DO $fixture$
BEGIN
IF NOT EXISTS(SELECT 1 FROM app.collections WHERE id='{ids['collection']}' AND user_id='{owner}' AND name='架空コレクション'
 AND research_instruction='架空データで設定の取得を確認する。' AND NOT enabled AND display_order=0 AND initial_lookback_mode='ai' AND initial_lookback_days IS NULL)
 OR NOT EXISTS(SELECT 1 FROM app.collection_schedules WHERE id='{ids['schedule']}' AND collection_id='{ids['collection']}' AND cron_expression='0 7 * * *')
 OR NOT EXISTS(SELECT 1 FROM app.watch_targets WHERE id='{ids['target']}' AND collection_id='{ids['collection']}' AND name='架空の調査対象' AND instruction='検証専用。外部へアクセスしない。' AND enabled AND display_order=0)
 OR NOT EXISTS(SELECT 1 FROM app.information_sources WHERE id='{ids['source']}' AND watch_target_id='{ids['target']}' AND name='架空の情報源' AND url='https://example.com/' AND instruction='取得処理は実行しない。') THEN
 RAISE EXCEPTION 'Fixture conflict';
END IF;
END $fixture$;
COMMIT;
"""
    query(sql, login)
    return ids


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--login-id", required=True)
    args = parser.parse_args()
    login = args.login_id.lower()
    if not re.fullmatch(r"[a-z0-9_-]{1,64}", login):
        raise SystemExit("Invalid fictional login ID.")
    seed(login)
    print("Fictional Collection is ready; existing settings were not overwritten.")


if __name__ == "__main__":
    main()
