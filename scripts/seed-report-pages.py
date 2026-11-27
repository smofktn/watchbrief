#!/usr/bin/env python3
"""Separate immutable fictional history for cursor pagination; no generation."""
import argparse
import hashlib
import json
from pathlib import Path
import runpy

ROOT = Path(__file__).resolve().parent.parent
original = runpy.run_path(str(ROOT / 'scripts/seed-reports.py'))


def fixture_rows(owner, count=35):
    rows = []
    first = None
    old_ids, old_rows = original['fixture_rows'](owner)
    for i in range(count):
        ids = {kind: '0' + hashlib.sha256(('watchbrief-report-pages-v1:'+owner+':'+str(i)+':'+kind).encode()).hexdigest()[:25].upper() for kind in old_ids}
        if first is None:
            first = ids
        replacements = dict(zip(old_ids.values(), ids.values()))
        # Same second with microsecond distinctions and a group of exact ties.
        micros = 40 if i % 7 == 0 else 35-i
        at = '2026-04-01T00:00:00.'+str(micros).zfill(6)+'Z'
        for table, old in old_rows:
            row = {}
            for key, value in old.items():
                if isinstance(value,str):
                    for before,after in replacements.items():
                        value = value.replace(before,after)
                row[key] = value
            if table=='generation_jobs':
                row['scheduled_at']=at
                row['collection_name']='追加取得用（架空）'+str(i)
                snapshot=json.loads(row['input_snapshot'])
                snapshot['fixture']='saved-report-pages-v1'
                row['input_snapshot']=json.dumps(snapshot,ensure_ascii=False)
            if table=='collections':
                row['name']='追加取得用（架空）'+str(i)
            rows.append((table,row))
    return first or {}, rows


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--login-id',required=True)
    args=parser.parse_args()
    original['seed'](args.login_id.lower(),rows_factory=fixture_rows)
    print('Fictional pagination history is ready; existing history was not overwritten.')


if __name__=='__main__':
    main()
