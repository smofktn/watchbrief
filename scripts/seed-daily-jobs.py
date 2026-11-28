#!/usr/bin/env python3
"""Immutable fictional Job history in a separate namespace; never runs generation."""
import argparse
import hashlib
import json
from pathlib import Path
import runpy

ROOT = Path(__file__).resolve().parent.parent
original = runpy.run_path(str(ROOT / 'scripts/seed-reports.py'))


def fixture_rows(owner, count=35, collection_id=None, collection_name=None):
    old_ids, old_rows = original['fixture_rows'](owner)
    rows = []
    first = None
    for i in range(count):
        ids = {kind: '0' + hashlib.sha256(('watchbrief-daily-v1:'+owner+':'+str(i)+':'+kind).encode()).hexdigest()[:25].upper() for kind in old_ids}
        if first is None:
            first = ids
        replacements = dict(zip(old_ids.values(), ids.values()))
        state = ('succeeded', 'queued', 'running', 'failed')[i % 4]
        # Page boundary stays on April 2 JST; final rows cross to April 1.
        at = '2026-04-01T15:00:00.'+str(40 if i % 7 == 0 else 35-i).zfill(6)+'Z' if i < 33 else '2026-04-01T14:59:59.999999Z'
        for table, old in old_rows:
            if state == 'queued' and table not in ('collections', 'watch_targets', 'information_sources', 'generation_jobs'):
                continue
            if state in ('running', 'failed') and table not in ('collections', 'watch_targets', 'information_sources', 'generation_jobs', 'generation_attempts'):
                continue
            row = {}
            for key, value in old.items():
                if isinstance(value, str):
                    for before, after in replacements.items():
                        value = value.replace(before, after)
                row[key] = value
            if table == 'collections':
                row['name'] = '日付別検証（架空）'+str(i)
            if table == 'generation_jobs':
                row.update(scheduled_at=at, collection_name=collection_name or '日付別検証（架空）'+str(i), status=state)
                if collection_id is not None:
                    row['collection_id'] = collection_id
                snapshot = json.loads(row['input_snapshot'])
                snapshot['fixture'] = 'daily-jobs-v1'
                row['input_snapshot'] = json.dumps(snapshot, ensure_ascii=False)
            if table == 'generation_attempts':
                row.update(status=state, finished_at=None if state == 'running' else row['finished_at'], failure_type='unexpected_error' if state == 'failed' else None)
            rows.append((table, row))
    return first or {}, rows


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--login-id', required=True)
    args = parser.parse_args()
    original['seed'](args.login_id.lower(), rows_factory=fixture_rows)
    print('Fictional daily history ready; no generation and no overwrite.')
