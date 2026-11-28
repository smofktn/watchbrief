# Collection別の保存済み予定・結果（Issue #90）

採用済みの利用者範囲を既存API/画面構造へ具体化する実装判断。`GET /api/collections/{id}/generation-jobs`（operationId: `listCollectionGenerationJobs`）は、認証済みUserが現在所有するCollectionの保存済みGenerationJobを返す。Collection IDは正規ULID。初回queryなし、続きは `?cursor={next_cursor}`。

## 認証・認可

Cookie sessionからUser IDを特定し、query検証より先に認証する。Collectionの不存在・他者所有・削除済みは同じ `404 not_found`。有効なCollectionにJobが0件の場合は `200` と空配列を返す。所有者IDをrequest/queryから受け取らない。Jobも `j.user_id` と `j.collection_id` の両条件で絞る。Collection選択画面の事前取得はAPI認可の代わりにならない。

## 正常応答

`200 OK` は既存[Job一覧API](get-api-generation-jobs.md)と同じJSON/summary/cursor形式とする。CollectionとJobの取得範囲だけを選択IDで限定する。

```json
{
  "generation_jobs": [
    {
      "id": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      "collection_name": "保存済み履歴名",
      "scheduled_at": "2026-11-27T03:04:05.123456Z",
      "status": "succeeded",
      "report_id": "01ARZ3NDEKTSV4RRFFQ69G5FAW"
    }
  ],
  "next_cursor": null
}
```

`report_id` はstatusが`succeeded`の場合だけ正規ULID、それ以外は`null`。Attempt、failure理由、input snapshot、User ID、本文は返さない。成功状態とReportの有無が一致しない行は内部障害として安全に閉じ、GETで修復しない。

## Query・順序

- `cursor`は省略または1個だけ。その他のquery、重複/空cursor、不正escape、512byte超、非正規cursorは `400 invalid_request`。未認証・失効は入力不正より先に `401 unauthenticated`。
- cursorは既存Job APIと同じJob専用`job1`形式を使う。UTC固定6桁microsecond・正規ULID・canonical base64urlを検証し、Frontendは解読/生成せず受け取った値をそのまま再送する。
- cursorはCollectionスコープ/認可を表さない。初回も全ての続きも `j.user_id` と指定された現在Collection IDをSQL条件に含める。別Collectionから得たcursorを使っても、そのCollectionのJobを返さない。
- `scheduled_at DESC, j.id DESC` のkeyset。初回/続きとも31件取得し30件返す。31件目がある場合は返した30件目のtupleを次境界とし、次回はtupleより厳密に小さい行から取得する。日時精度を丸めずUTC microsecondで扱う。
- 全ページのsnapshotは保証しない。更新で初期30件から現在状態を再取得する。

## 削除との競合・保存権限

Owned Collectionの存在判定とページ取得を単一SQL文の同一スナップショットで行う。Collection削除の前後どちらかに整合した結果となる。Job側は `user_id` と `collection_id` を両方照合する。Job起点のLEFT JOINでAttempt未作成のqueued行を落とさない。削除済み履歴はこのAPIへ含めず、既存の日付別APIでJob.user_idにより認可する。

既存テーブル・SELECT権限・`generation_jobs(collection_id, scheduled_at)`一意索引と`generation_jobs(user_id, scheduled_at, id)`索引を使う。migration、権限、DB保存変更は追加しない。

## 応答・ログ

| Status | Code | 意味 |
|---|---|---|
| 200 | — | 有効な本人所有CollectionのJob一覧。空履歴は空配列 |
| 400 | `invalid_request` | 認証後の不正ID/query/cursor |
| 401 | `unauthenticated` | sessionなし/失効。query検証より優先 |
| 404 | `not_found` | Collection不存在・他者所有・削除済み |
| 500 | `internal_error` | DB障害、timeout、保存内容不整合 |

全応答に`Cache-Control: no-store`。GETは書き込みとCookie更新をしない。内部SQL原因・cursor・Collection名・ID・Job内容をエラーレスポンス/ログへ出さず、固定イベントとoperation/stage/result/versionだけを記録する。正常JSONは上記契約どおりのCollection名・ID・Job summaryを返す。

## 確認観点

- 401が不正query/cursorより優先される。
- 本人・他者・不存在・削除済みCollectionと、存在する空履歴を区別する。
- CollectionとJob両所有者条件、削除後の日付別履歴、queuedを含むLEFT JOINを確認する。
- 0/30/31/35件、microsecond順序、別Collection cursor、同一SQLスナップショット、Report有無不整合を確認する。
- DB障害/timeout時に安全な500となり、保存履歴・状態・順序を変更しない。

[Collection別画面](../画面仕様/Collection別の予定・結果一覧.md)、[既存Job API](get-api-generation-jobs.md)、[Report詳細API](get-api-reports.md)を参照する。
