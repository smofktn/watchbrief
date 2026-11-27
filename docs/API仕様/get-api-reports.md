# 保存済みReport取得

Issue #57の委任に基づく実装判断。Cookie認証、Job.user_idによる所有者限定、Collection削除後の保持と本文のDB保存は既存合意を具体化する。

| operationId | メソッド・パス | 正常JSON |
|---|---|---|
| listReports | `GET /api/reports` | `{ "reports": [ReportSummary], "next_cursor": string|null }`。最大30件、空は `[]` |
| getReport | `GET /api/reports/{id}` | Reportオブジェクト |

ReportSummaryは `id`（ULID）、`collection_name`（Job時点の履歴名）、`scheduled_at`（予定時刻）、`generated_at`（成功Attempt.finished_at）を持つ。時刻はRFC3339文字列。一覧はscheduled_at降順・id降順とし、本文・snapshot・所有者IDは返さない。成功Report一覧を30件ずつcursorで追加取得する。日付別のJob状態一覧は[Job一覧API](get-api-generation-jobs.md)で扱い、Collection別は後続とする。

詳細はSummaryの4項目に `body_markdown`、`summary_markdown`（保存テキスト）、`updates_started_at`、`updates_ended_at`（Attempt.started_at）を追加する。構造化子テーブルや入力snapshotの公開APIは追加しない。本文は閲覧時に組み立て直さない。リクエストbodyは使用しない。詳細のquery契約は変更しない。一覧のqueryは次節を参照。

HTTP層で同名Cookieの重複・不正トークンを拒否し、ユースケースがセッション由来User IDを取得する。SQLはReport→Attempt→Jobを結び、User IDをWHERE条件に含める。入力からUser IDを指定させない。Collectionは認可に使わない。

| 状態 | HTTP | error.code |
|---|---|---|
| Cookieなし/不正/無効なセッション | 401 | unauthenticated |
| 認証後にIDが正規ULIDでない | 400 | invalid_request |
| 他UserのReport・不存在 | 404 | not_found（同じ本文） |
| DB失敗・timeout・想定外 | 500 | internal_error |

成功・失敗とも `Cache-Control: no-store`。エラーは[共通JSON](README.md)を使う。DB処理は10秒で中断する。ログの `report_read_failed` と固定のoperation/stage/result/versionのみで障害を記録し、本文、設定、トークン、ハッシュ、内部DB原因を出さない。GETは更新を行わずCookieを更新しない。既存のCookie検証・セッション照合を再利用する。

確認観点は、本人の一覧/詳細、空配列、他者混入なし、同一404、Collection削除後の所有者認可、不正ID、Cookie・DB失敗の分類、ログ秘密情報の除外、保存済み本文の一致とする。UIは[保存済みReportの取得](../画面仕様/保存済みReportの取得.md)を参照する。

## Markdown表示との境界

APIはbody_markdownをHTMLへ変換せず、保存済みの同じ文字列を返す。閲覧によるDB本文・履歴の更新、構造化データからの再組立ては行わない。ブラウザの[安全なMarkdown表示](../画面仕様/保存済みReportの取得.md#安全なmarkdown表示)だけを追加する。所有者404・認証・応答型・no-storeは維持する。

## 30件ずつのcursor追加取得

上ペインの代理判断を具体化する。初回はqueryなし、続きは `?cursor={サーバーのnext_cursor}`。外部limit指定なし。DBで31件まで読み、30件を返す。0件/30件ちょうどならnext_cursor:null、31件以上なら返した30件目の境界を返す。本文/snapshot/所有者IDは一覧に含めない。並びはscheduled_at DESC, report.id DESC、続きは境界tupleより厳密に小さい行とする。

cursorは固定文字列 `1|YYYY-MM-DDTHH:mm:ss.ffffffZ|ULID` のUTF-8（56byte）をpaddingなしbase64urlへ符号化した75文字。日時はUTC/固定6桁小数でDBマイクロ秒を保持し、正規日時（年0001〜9999）/正規ULID/version/再エンコード一致を検証する。署名/新Secretは追加せず、cursorを認可として扱わない。所有者は常にセッション→Job.user_idのWHEREで絞る。不正/他者の境界でも他者情報を返さない。

認証とセッション有効性照合を先に行い、未認証/失効は不正query/cursorより401を優先する。認証後、queryはcursor1個だけを許可し、未知/重複/空/不正escape/512byte超・不正cursorは400 invalid_request。同じcursorは同じ境界の読み取りとして再試行できる。GETで履歴・本文・予定時刻を更新しない。ログへcursor/query/本文/秘密値を出さない。

先頭へ追加された分は初期再読込で確認する。offsetのずれによる重複を避けるが、全ページの厳密DB snapshotは保証しない。新しい保存が境界の前後へ確定する時点により、この続きで見える/見えない場合がある。画面は重複/逆順等の不整合payloadを追加せず、既存項目と境界を保持する。
