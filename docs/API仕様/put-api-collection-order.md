# PUT /api/collections/order — Collection表示順の一括保存（Issue #96）

## 目的と採用状況

ホームで本人の現在Collection全体を並べ替えて保存する。表示順変更の範囲、同値時ID順、一意制約を設けないことは[プロダクト仕様](../product-definition.md#ホームからのcollection表示順変更issue-96)に従う。以下のHTTP/JSON・要求番号・エラー分類は範囲を実現する通常の実装契約であり、利用者が個々の通信細部を直接選んだこととは区別する。

## 認証・要求

`PUT /api/collections/order` は共通APIのCookie/session・Origin/CSRF保護を適用し、User IDを本文から受け取らない。session由来Userの全Collectionだけを読む。認証・CSRFを既存の書込APIと同じ方式で検証し、未認証UserではCollectionの有無を調べない。

`Content-Type: application/json` と小文字UUID v4の `Idempotency-Key` を各操作に必須とする。本文は1MiB以下の厳密JSONで、未知/重複/欠落key、型不正、末尾の別JSON、不正Unicodeを拒否する。Collection総数の業務上限は設けない。

```json
{
  "expected": [
    {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAV", "version": "3"},
    {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAW", "version": "1"}
  ],
  "ordered_ids": [
    "01ARZ3NDEKTSV4RRFFQ69G5FAW",
    "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  ]
}
```

`expected` は保存前の完全な本人Collection一覧で、表示順（`display_order,id`）の並びを維持する。各idは正規ULID、versionは既存PATCHと同じ1〜9223372036854775807の正規十進文字列である。`ordered_ids` は保存後の完全な順序であり、expectedと同一集合・同じ件数を持つ正規ULIDの重複なし配列とする。0件は両方空、1件は同じ1 IDを含む。

各配列内の重複、IDの形式不正、配列同士の件数/集合不一致、正規version範囲外は400 `invalid_request` とする。形式上有効なexpectedと現在の本人Collection一覧に違いがある場合、またはexpected内のIDに他者所有/不存在が含まれ現在一覧と一致しない場合は一律409 `order_conflict` とし、所有情報を区別しない。

## 保存・再試行・競合

サーバーは検証済み入力を固定フィールド順・配列順のJSONへ直列化したSHA-256 digestを作る。所有Userごとの `hashtextextended(user_id,18)` transaction advisory lockを取得し、Read Committed transactionで要求記録を照合する。同じUser/key/digestの既存記録は以前のcommit成功を意味し、現在状態を再更新せず204を返す。同じkeyでdigestが異なる場合は409 `order_request_conflict`。

新しいkeyでは、現在の所有Collection全体を `display_order,id` 順に読み、expectedのID順・件数・各versionと完全一致することを確認する。1件でも追加・削除・改名・編集・順序変更等があり完全一致しなければ409 `order_conflict`。期待IDに他者所有または不存在のIDが含まれる場合も同じ現在値不一致として409にし、所有情報を区別しない。期待状態が現在値と一致しない場合は更新しない。

初回要求は本人の希望順を0から連番の `display_order` として一括更新する。値が変わるCollectionだけ `display_order`・Collection全体 `version`・`updated_at` を更新し、変更なし行は版を増やさない。空/1件や既に同じ順も成功扱いとする。版上限・DB障害・commit失敗は全rollbackする。順序変更と本人/key/digest要求記録のINSERTを同一transactionで確定し、要求記録を先に保存して順序だけ後から失敗する状態を作らない。

新規作成と本体編集で使うowner lock namespace 18を共有する。現行にはCollection削除APIがない。将来削除を実装する場合もこのlockを共有し、順序更新との欠落/復活競合を直列化する。既存 `collection_creation_requests` と `collection_update_requests` のdigest/結果/同一要求再送契約は変更・流用しない。

## 応答・エラー

成功（初回・同じ操作の再送とも）: `204 No Content`、本文なし。全応答は `Cache-Control: no-store`。Cookie期限更新を伴わない。

| HTTP | `error.code` | 意味 |
|---|---|---|
| 400 | `invalid_request` | key・JSON・ID・version・配列条件が不正 |
| 401 | `unauthenticated` | session欠落・不正・失効 |
| 403 | `csrf_rejected` | 共通API契約のOrigin/CSRF拒否 |
| 409 | `order_conflict` | 形式上有効なexpectedと現在の本人一覧の順序・構成・versionが異なる。入力を保持して最新状態を確認し、明示的に再編集または取消する |
| 409 | `order_request_conflict` | 同じ本人/keyで異なる入力digest |
| 500 | `internal_error` | 保存・commitの成否が確定しない場合を含む。元のkey/同じ本文で手動再試行する |

エラー本文・ログにCollection名/ID、要求入力、要求番号、認証情報、DB原因を含めない。`order_conflict` / `order_request_conflict` は確定応答であり、同じ要求を再送しても競合は解消しない。画面は入力順を保持して現在状態を確認させ、利用者が明示的に再編集/再読込した場合だけ新しい操作keyを使う。500・timeout・通信断・不正な成功応答は結果不明として未保存と断定せず、同じkey・同じ本文を再送する。

このAPIは現在設定の表示順だけを変更する。Collection名/ON-OFF/targets/sources/schedule、GenerationJob/Attempt/Report/input_snapshotや履歴を変更せず、生成・有効化・削除・外部取得を開始しない。

## 確認観点

0/1/複数件・同値display_orderのID安定順、空・重複・欠落・不正/他者ID、所有者分離、認証/CSRF、不正要求、同じkey同じdigest/別digest、commit後の応答喪失と再送、並行同key、別要求・作成/編集競合、rollbackと復旧、版増加行、設定/履歴不変、DB列権限、画面の入力保持・取消・再取得・logout/epoch遅延破棄を確認する。専用隔離DBのみmigrationを適用し、通常DBへ適用しない。
