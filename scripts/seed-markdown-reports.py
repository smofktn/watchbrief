#!/usr/bin/env python3
"""Add a separate immutable fictional Markdown Report; never rewrite old history."""
import argparse
import hashlib
import json
from pathlib import Path
import runpy

ROOT = Path(__file__).resolve().parent.parent
original = runpy.run_path(str(ROOT / 'scripts/seed-reports.py'))


def fixture_rows(owner):
    old_ids, old_rows = original['fixture_rows'](owner)
    ids = {kind: '0' + hashlib.sha256(('watchbrief-markdown-fixture-v1:' + owner + ':' + kind).encode()).hexdigest()[:25].upper() for kind in old_ids}
    substitutions = dict(zip(old_ids.values(), ids.values()))
    rows = []
    for table, old_row in old_rows:
        row = {}
        for key, value in old_row.items():
            if isinstance(value, str):
                for before, after in substitutions.items():
                    value = value.replace(before, after)
            row[key] = value
        if table == 'generation_jobs':
            snapshot = json.loads(row['input_snapshot'])
            snapshot['fixture'] = 'saved-report-markdown-v1'
            row['input_snapshot'] = json.dumps(snapshot, ensure_ascii=False)
        if table in ('collections', 'generation_jobs'):
            row['name' if table == 'collections' else 'collection_name'] = 'Markdown表示確認用（架空）'
        if table == 'reports':
            row['body_markdown'] = '''# 架空Markdown Report

## 現在の状態

段落の **重要な事実** と *補足*、`inline <code>`。

- 架空の箇条書き
- 追加の項目

3. 順序を持つ項目
4. 次の項目

```html
<img src="https://external.invalid/code-image" onerror="window.reportExecuted=true">
```

    indented <code>

[安全な架空出典](https://example.com/news?q=公開情報#発表)

[空白escapeの出典](https://example.com/a%20b)

[拒否scheme](javascript:alert(1))
[拒否相対](/relative)
[拒否protocol相対](//external.invalid/relative)
[拒否fragment](#app)
[拒否userinfo](https://user:pass@external.invalid/)
[拒否空userinfo](https://@external.invalid/)
[拒否control](https://external.invalid/%C2%85)
[拒否entity](jav&#x61;script:alert(1))

![画像の代替テキスト](https://external.invalid/image.png)

<script>window.reportExecuted=true</script>
<img src="https://external.invalid/raw-image" onerror="window.reportExecuted=true">
<iframe src="https://external.invalid/frame"></iframe>
<svg onload="window.reportExecuted=true"></svg>
<math><mtext>架空</mtext></math>
<object data="https://external.invalid/object"></object>
<video src="https://external.invalid/video"></video>
<style>body{background:url(https://external.invalid/style)}</style>
<a id="location" name="x" href="javascript:alert(1)" ping="https://external.invalid/ping">生HTML</a>
'''
        rows.append((table, row))
    return ids, rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--login-id', required=True)
    args = parser.parse_args()
    original['seed'](args.login_id.lower(), rows_factory=fixture_rows)
    print('Fictional Markdown Report is ready; existing history was not overwritten.')


if __name__ == '__main__':
    main()
