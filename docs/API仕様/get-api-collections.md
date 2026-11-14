# Collection設定の取得（Issue #56）

## 目的と採用状況

認証済みUserが自分の保存済みCollection設定を読む。#17の順次実装の委任に基づく実装判断として、一覧を `GET /api/collections`、詳細を `GET /api/collections/{id}` とした。本体の追加は[作成API](post-api-collections.md)、変更・削除等は#18の後続範囲で扱う。

認証は[共通のCookie方式](README.md#共通の認証と秘密情報)を使う。APIがCookieのハッシュから所有UserのIDを取得し、そのIDをSQLの所有者条件へ渡す。リクエストに指定された `user_id` を所有者として使わず、レスポンスにも所有User IDや認証情報を含めない。単なるULIDの推測困難性を認可の代わりにしない。

## リクエストとレスポンス

両方ともリクエスト本文なし。詳細の `id` は先頭0〜7の正規大文字ULID。クエリによる絞り込み・ページングは今回追加しない。

一覧の200は、`display_order`、同値では `id` の順に並べた `collections` 配列。0件はnullではなく空配列。

```json
{
  "collections": [
    {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAV", "name": "架空コレクション", "enabled": false, "display_order": 0}
  ]
}
```

詳細の200は次の構造。設定値の文字列は表示用のテキストであり、HTMLとして挿入しない。情報源URLを取得したり、自動で外部へ通信したりしない。

```json
{
  "id": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "name": "架空コレクション",
  "enabled": false,
  "display_order": 0,
  "version": "1",
  "research_instruction": "架空の調査方針",
  "initial_lookback_mode": "ai",
  "initial_lookback_days": null,
  "schedule": {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAW", "cron_expression": "0 7 * * *"},
  "targets": [
    {
      "id": "01ARZ3NDEKTSV4RRFFQ69G5FAX",
      "name": "架空の対象",
      "instruction": "補足指示",
      "enabled": true,
      "display_order": 0,
      "sources": [
        {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAY", "name": "架空の情報源", "url": "https://example.com/", "instruction": "取得しない"}
      ]
    }
  ]
}
```

Scheduleなしは `schedule: null`、対象・情報源なしはそれぞれ空配列。対象は `display_order,id`、情報源は `id` の順。Scheduleのcron式は日本時間として表示し、今回スケジュールを実行しない。

## エラーと副作用

| HTTP | error.code | 意味 |
|---|---|---|
| 401 | unauthenticated | Cookie欠落・不正・重複、対応セッションなし |
| 400 | invalid_request | 認証後の詳細IDが不正 |
| 404 | not_found | 詳細が存在しない、または他Userの所有。本文も同じ |
| 500 | internal_error | 認証照合・設定取得等の内部障害。内部原因・設定本文は返さない |

認証を確認してから詳細ID・所有者を扱う。全応答は `Cache-Control: no-store`、JSON形式。GETによる設定変更・User/セッション作成は行わない。Cookieの期限更新は従来どおり本人情報APIで行い、このAPIではSet-Cookieを発行しない。

この取得経路は設定4テーブルのSELECTを使う。#18で追加する本体INSERTと要求記録の権限は[作成API](post-api-collections.md)を参照する。詳細は所有者で絞ったCollectionを起点に、外部キーで所属する配下設定だけを組み立てる。ID差し替え・配下情報の他者混入・空配列・未認証と内部障害の区別を試験する。構造化エラーログは固定項目とし、Cookie・DBエラー・設定全文を含めない。

本体編集で使う `version` は詳細JSONだけに含む正規十進文字列。版の照合と[更新API](patch-api-collections.md)を組み合わせる。一覧の応答は変更しない。版10の明示migrationが必要。
