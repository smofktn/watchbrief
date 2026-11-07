# 保存済みReport取得

Issue #57の委任に基づく実装判断。Cookie認証、Job.user_idによる所有者限定、Collection削除後の保持と本文のDB保存は既存合意を具体化する。

| operationId | メソッド・パス | 正常JSON |
|---|---|---|
| listReports | `GET /api/reports` | `{ "reports": [ReportSummary] }`。空は `[]` |
| getReport | `GET /api/reports/{id}` | Reportオブジェクト |

ReportSummaryは `id`（ULID）、`collection_name`（Job時点の履歴名）、`scheduled_at`（予定時刻）、`generated_at`（成功Attempt.finished_at）を持つ。時刻はRFC3339文字列。一覧はscheduled_at降順・id降順とし、本文・snapshot・所有者IDは返さない。日付/Collectionの絞込やページングは最小経路に追加せず #18で検討する。大量履歴での負荷はこの初期実装の制限となる。

詳細はSummaryの4項目に `body_markdown`、`summary_markdown`（保存テキスト）、`updates_started_at`、`updates_ended_at`（Attempt.started_at）を追加する。構造化子テーブルや入力snapshotの公開APIは追加しない。本文は閲覧時に組み立て直さない。リクエストbody・queryパラメータは使用しない。

HTTP層で同名Cookieの重複・不正トークンを拒否し、ユースケースがセッション由来User IDを取得する。SQLはReport→Attempt→Jobを結び、User IDをWHERE条件に含める。入力からUser IDを指定させない。Collectionは認可に使わない。

| 状態 | HTTP | error.code |
|---|---|---|
| Cookieなし/不正/無効なセッション | 401 | unauthenticated |
| 認証後にIDが正規ULIDでない | 400 | invalid_request |
| 他UserのReport・不存在 | 404 | not_found（同じ本文） |
| DB失敗・timeout・想定外 | 500 | internal_error |

成功・失敗とも `Cache-Control: no-store`。エラーは[共通JSON](README.md)を使う。DB処理は10秒で中断する。ログの `report_read_failed` と固定のoperation/stage/result/versionのみで障害を記録し、本文、設定、トークン、ハッシュ、内部DB原因を出さない。GETは更新を行わずCookieを更新しない。既存のCookie検証・セッション照合を再利用する。

確認観点は、本人の一覧/詳細、空配列、他者混入なし、同一404、Collection削除後の所有者認可、不正ID、Cookie・DB失敗の分類、ログ秘密情報の除外、保存済み本文の一致とする。UIは[保存済みReportの取得](../画面仕様/保存済みReportの取得.md)を参照する。
