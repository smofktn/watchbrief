#!/usr/bin/env python3
"""Atomically add fictional successful history and a saved Report for one local User."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import runpy

ROOT = Path(__file__).resolve().parent.parent
query = runpy.run_path(str(ROOT / 'scripts/seed-collections.py'))['query']


def fixture_id(owner, kind):
    return '0' + hashlib.sha256(('watchbrief-report-fixture-v3:' + owner + ':' + kind).encode()).hexdigest()[:25].upper()


def literal(value):
    if value is None:
        return 'NULL'
    if isinstance(value, bool):
        return 'true' if value else 'false'
    if isinstance(value, int):
        return str(value)
    return "'" + str(value).replace("'", "''") + "'"


def fixture_rows(owner):
    ids = {kind: fixture_id(owner, kind) for kind in ('collection', 'target', 'source', 'job', 'attempt', 'report', 'result', 'fact', 'current_fact', 'report_source', 'fact_source', 'current_fact_source', 'failure')}
    # Fixture v3 is a new immutable history namespace. Never rewrite earlier verification history.
    research = '外部取得を行わない検証用。'
    target_instruction = '架空データだけで現在状態と更新を確認する。'
    source_instruction = '外部へアクセスしない。'
    source_name, source_url = '架空の出典', 'https://example.com/'
    target_status = '重要な変化あり'
    current_state = '検証用の架空データです。'
    update = '架空の更新を確認しました。'
    ai_summary = '架空情報の整理。'
    failure_reason = '検証用の取得失敗。外部取得は行っていない。'
    snapshot = {
        'schema_version': 1, 'fixture': 'saved-report-v3', 'collection_id': ids['collection'],
        'research_instruction': research, 'initial_lookback_mode': 'ai', 'initial_lookback_days': None,
        'targets': [{'id': ids['target'], 'name': '架空の対象', 'instruction': target_instruction, 'display_order': 0,
                     'sources': [{'id': ids['source'], 'name': source_name, 'url': source_url, 'instruction': source_instruction}]}],
    }
    body = ('# 架空の保存済みReport\n\n今回の変化：' + update
            + '\n\n## 架空の対象\n状態：' + target_status
            + '\n\n現在状態：' + current_state + '\n出典：' + source_url
            + '\n\n更新・新着：' + update + '\n出典：' + source_url
            + '\n\nAIによる整理：' + ai_summary
            + '\n\n取得失敗：架空の取得失敗 https://example.invalid/'
            + '\n\n<script>window.reportExecuted = true</script>')
    # This dedicated stopped Collection is independent of the Collection settings fixture.
    rows = [
        ('collections', dict(id=ids['collection'], user_id=owner, name='保存Report確認用（架空）', research_instruction=research, enabled=False, initial_lookback_mode='ai', initial_lookback_days=None, display_order=0)),
        ('watch_targets', dict(id=ids['target'], collection_id=ids['collection'], name='架空の対象', instruction=target_instruction, enabled=True, display_order=0)),
        ('information_sources', dict(id=ids['source'], watch_target_id=ids['target'], name=source_name, url=source_url, instruction=source_instruction)),
        ('generation_jobs', dict(id=ids['job'], user_id=owner, collection_id=ids['collection'], collection_name='保存Report確認用（架空）', input_snapshot=json.dumps(snapshot, ensure_ascii=False), scheduled_at='2026-01-02T00:00:00Z', queued_at='2026-01-02T00:00:01Z', status='succeeded')),
        ('generation_attempts', dict(id=ids['attempt'], generation_job_id=ids['job'], attempt_number=1, started_at='2026-01-02T00:00:02Z', heartbeat_at='2026-01-02T00:00:03Z', finished_at='2026-01-02T00:00:04Z', status='succeeded', failure_type=None)),
        ('reports', dict(id=ids['report'], generation_attempt_id=ids['attempt'], body_markdown=body, summary_markdown=update, updates_started_at='2026-01-01T00:00:00Z')),
        ('report_target_results', dict(id=ids['result'], report_id=ids['report'], target_id=ids['target'], target_name='架空の対象', display_order=0, status=target_status, ai_summary_markdown=ai_summary)),
        ('report_facts', dict(id=ids['current_fact'], report_target_result_id=ids['result'], fact_kind='current_state', content_markdown=current_state, display_order=0)),
        ('report_facts', dict(id=ids['fact'], report_target_result_id=ids['result'], fact_kind='update', content_markdown=update, display_order=1)),
        ('report_sources', dict(id=ids['report_source'], report_target_result_id=ids['result'], source_id=ids['source'], source_kind='registered', source_name=source_name, source_url=source_url)),
        ('fact_sources', dict(id=ids['current_fact_source'], report_fact_id=ids['current_fact'], report_source_id=ids['report_source'])),
        ('fact_sources', dict(id=ids['fact_source'], report_fact_id=ids['fact'], report_source_id=ids['report_source'])),
        ('report_source_failures', dict(id=ids['failure'], report_target_result_id=ids['result'], source_id=None, source_kind='discovered', source_name='架空の取得失敗', source_url='https://example.invalid/', reason=failure_reason)),
    ]
    return ids, rows


def seed(login):
    if not re.fullmatch(r'[a-z0-9_-]{1,64}', login):
        raise SystemExit('Invalid fictional login ID.')
    owner = query("SELECT id FROM app.users WHERE login_id=:'fixture_login';", login)
    if not re.fullmatch(r'[0-7][0-9A-HJKMNP-TV-Z]{25}', owner):
        raise SystemExit('Create the fictional User with make user-create first.')
    ids, rows = fixture_rows(owner)
    sql = ['BEGIN;', 'SET LOCAL ROLE watchbrief_migrator;']
    checks = []
    for table, row in rows:
        sql.append(f"INSERT INTO app.{table}({','.join(row)}) VALUES ({','.join(literal(v) for v in row.values())}) ON CONFLICT(id) DO NOTHING;")
        # Column comparisons infer real DB types, including timestamps and JSONB.
        checks.append(f"NOT EXISTS (SELECT 1 FROM app.{table} WHERE " + ' AND '.join(f'{k} IS NOT DISTINCT FROM {literal(v)}' for k, v in row.items()) + ')')
    sql.append('DO $fixture$ BEGIN IF ' + ' OR '.join(checks) + " THEN RAISE EXCEPTION 'Fixture conflict'; END IF; END $fixture$;")
    sql.append('COMMIT;')
    query('\n'.join(sql), login)
    return ids


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--login-id', required=True)
    args = parser.parse_args()
    seed(args.login_id.lower())
    print('Fictional saved Report is ready; existing values were not overwritten.')


if __name__ == '__main__':
    main()
