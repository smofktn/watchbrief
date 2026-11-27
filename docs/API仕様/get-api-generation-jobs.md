# 保存済みGenerationJob一覧（Issue #88）

採用された利用者体験を具体化する担当の実装判断。`GET /api/generation-jobs`（operationId: listGenerationJobs）は本人の保存済み予定・結果を返す。初回queryなし、続きは `?cursor={next_cursor}`。認証・セッション照合をquery検証より先に行う。

正常200は `{ "generation_jobs": [GenerationJobSummary], "next_cursor": string|null }`。Summaryは `id`（Job正規ULID）、`collection_name`（Job時点の名前）、`scheduled_at`（UTCのRFC3339、最大6桁小数）、`status`（queued/running/succeeded/failed）、`report_id`（成功時だけ正規ULID、それ以外null）。Attempt・failure理由・snapshot・所有者ID・本文は返さない。空は配列 `[]`。

所有者はCookieのセッション→Job.user_idのWHEREで制限し、削除済みCollectionも保持する。Attempt/ReportはLEFT JOINし、Jobのない日・現在cronからの復元を行わない。成功とReport有無の不整合は500内部障害。GETは書込/修復/Cookie更新をしない。

scheduled_at DESC, Job.id DESCのkeysetで31件取得し最大30件返す。31件目があるとき返した30件目を次境界とし、続きはtupleより厳密に小さい行。全ページsnapshotは保証せず、新しい先頭や状態変化は手動更新で読む。

cursorは `job1|YYYY-MM-DDTHH:mm:ss.ffffffZ|ULID` の59byteをpaddingなしbase64urlにした79文字。成功Report一覧のv1 cursorと相互拒否する。UTC固定6桁、年0001〜9999、正規日時/ULID/version/再エンコード一致を検証。cursorはSecret/認可ではなく常に所有者条件を適用し、署名/新Secretは追加しない。Frontendはそのまま再送し解読/再生成しない。

認証後queryはcursor1個だけ許可。未知/重複/空/不正escape/512byte超・不正cursorは400 invalid_request。未認証/失効は不正queryより401 unauthenticatedを優先する。DB失敗/10秒timeout/不整合は500 internal_error。共通エラーJSON、全応答no-store。固定ログ `generation_jobs_read_failed` とoperation/stage/result/versionだけを使いquery/cursor/設定/内部DB原因/秘密値を出さない。

[日付別画面](../画面仕様/日付別の予定・結果一覧.md)と[既存Report API](get-api-reports.md)を参照する。
