# GET /api/home — ホーム取得（Issue #92）

## 目的と採用状況

認証済みUserの現在Collectionと、各Collectionに保存済みの最新成功Reportへの参照を読む。保護画面 `/home` から利用する。業務上の範囲は[プロダクト仕様](../product-definition.md#ホームの最初の経路issue-92)、読み取り設計は[DB設計](../database-design.md#ホームの最新保存済みreportissue-92)を参照する。

`GET /api/home`、応答JSON、最新判定の `scheduled_at DESC, report.id DESC` は対象範囲を満たす通常の実装契約として採用する。利用者が個々のtransport細部を選択したという意味ではない。

## リクエスト

本文・path parameter・query parameterは使わない。認証UserはCookieのセッションから決定し、リクエストでUser IDを受け取らない。認証を確認してから、非対応query等を400として扱う。

## 正常レスポンス

`200 OK`。Collectionは現在設定の `display_order ASC, id ASC` 順。ReportがまだないCollectionも行に含め、`latest_report_id` は `null` とする。現在Collectionが0件なら空配列を返す。

```json
{
  "collections": [
    {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAV", "name": "現在のCollection名", "latest_report_id": "01ARZ3NDEKTSV4RRFFQ69G5FAY"},
    {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAW", "name": "未生成のCollection", "latest_report_id": null}
  ]
}
```

`id` は現在Collectionの正規ULID、`name` は現在の設定名、`latest_report_id` は本人・同一Collectionの保存済み成功Report IDまたはnull。Job時点の履歴名はこの応答へ含めない。本文、summary、input_snapshot、Attempt、予定時刻、所有者IDも返さない。Report詳細は既存の `GET /api/reports/{id}` とその認可・履歴名表示を使う。

最新Reportは同じCollection内で、成功Job・成功Attempt・保存済みReportの組に絞った後、`scheduled_at DESC, report.id DESC` の先頭とする。これは既存Report一覧の順序にそろえる。したがって、その後に作られたqueued/running/failed Jobや失敗Attemptは、既存の成功Reportを隠さない。

## エラー・副作用

| HTTP | `error.code` | 意味 |
|---|---|---|
| 401 | `unauthenticated` | Cookie欠落・不正・無効セッション |
| 400 | `invalid_request` | 非対応query等。認証後に判定 |
| 500 | `internal_error` | User照合・ホーム読取の内部障害 |

エラー応答の詳細は[共通JSON](README.md#共通の応答案)に従う。全応答は`Cache-Control: no-store`、成功はJSONとする。GETはCollection・Job・Attempt・Report・セッションを作成/変更せず、Cookie期限も更新しない。ログにCookie、Report本文、snapshot、Collection名/ID、DB原因を記録しない。

現在Collectionを起点に、所有Collection行と各Collectionの最新Report参照を1つのSQL statementで取得する。Collection側とJob側の両方へセッション由来User IDを適用する。Collection削除と読み取りが並行した場合は、そのSQL statementのsnapshotとして整合した現在Collection一覧を返す。

## 確認観点

本人/他者分離、Collection 0件、未生成Collection、複数Collectionの順序、Collection改名とJobの履歴名の分離、削除後非表示、Reportなしのnull、詳細リンクの既存所有者認可、400/401優先、DB障害、履歴不変、本文/snapshot/秘密値の非出力を検証する。Job・Attempt双方の成功条件を確認し、先行する成功Reportの後に新しいqueued/running/failed Job/Attemptを追加しても成功Reportへの参照が残ることを検証する。異なるCollectionに同じ予定時刻のReportを用意し、最新参照が相互に混ざらないことも確認する。現在は `(collection_id, scheduled_at)` が一意のため同一Collection内の同時刻Reportは作成できないが、既存一覧との整合のためReport IDも降順の決定的なtie-breakerに含める。大規模履歴性能の保証はしない。
