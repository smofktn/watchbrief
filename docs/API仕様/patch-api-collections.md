# Collection本体・WatchTarget・情報源の更新（Issue #74・#78・#80）

## 採用状況と範囲

版番号と更新要求番号で、先行更新との競合と応答喪失後の再試行を区別する方式に合意した。`PATCH /api/collections/{ULID}` は停止中Collectionの名前・任意の調査方針・初回確認期間を一体として変更する。#78で任意のWatchTarget変更を同じ保存へ追加した。以下の厳密JSON・版の物理型・エラーコード・保存構造は、合意を具体化する実装判断である。

認証・CSRFは[共通API仕様](README.md)に従う。所有者はセッションから決め、他Userと不存在は本文も同じ404とする。対象の追加・名前と補足指示の編集は本APIで扱う。情報源の追加・編集は末尾の#80拡張で扱う。削除・ON/OFF操作・有効化・並べ替えは追加しない。

## 要求

`Content-Type: application/json` と1つの `Idempotency-Key` を必須とする。更新要求番号は[作成API](post-api-collections.md)と同じ小文字UUID v4形式で、作成要求とは別の記録に保存する。Collection ID・要求番号・版は認証情報の代わりにしない。

```json
{
  "name": "架空の新しい名前",
  "research_instruction": "任意の調査方針",
  "initial_lookback_mode": "days",
  "initial_lookback_days": 30,
  "version": "1"
}
```

本体4項目とversionの5項目を必須とし、targets・sources変更配列は任意とする。本体4項目の値・文字数・Unicode・空白・NUL・日数の扱いは[作成APIの入力条件](post-api-collections.md#要求)を維持する。任意方針は空文字、AI方式の日数はnull。versionは取得済みの編集元の版で、1〜9223372036854775807の正規十進文字列（符号・先頭0なし）。JSON数値は拒否し、ブラウザの整数精度に依存させない。

1MiBの本文上限、不正UTF-8・不対サロゲート・未知/重複/欠落項目・型不正・末尾の別JSONを拒否する。enabled・display_order・所有者を受け取らない。配下は後述のtargets・sources変更配列を受け付ける。

## 保存と成功応答

既存のCSRF・Cookie形式・厳密JSON検査の後、ユースケースがセッション由来の所有者と入力を検証する。DBは本人の作成処理と共有するtransaction advisory lockを取り、所有Collectionの存在、更新要求記録、停止状態、版を調べる。同じ所有者の別Collectionに対する同一要求番号の競合も直列化する。

初回は所有者・ID・版一致・停止中を条件に本体4列、版、updated_atをUPDATEし、版を1増やす。指定された対象変更も適用し、更新結果と要求記録を同じtransactionで確定し、片側だけ残さない。既存本体、配下設定、履歴を単一SQLで読む取得契約は維持する。将来の別の更新処理も版の増加を守る必要がある。

初回と同じ要求番号・同じ本文なら、元の保存結果を200で返す。入力一致はID・編集元版・本体4項目と、存在する場合の対象変更を固定構造のJSONへ直列化したSHA-256で判定する。再送では版やupdated_atを更新しない。

```json
{"id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","version":"2"}
```

versionはその要求が保存した版であり、最新の版を保証しない。別更新後の旧要求を再送しても、設定を元へ戻さず保存済み結果を返す。画面は200を確認したら編集を終え、詳細GETで現在値を読む。全応答はJSON・`Cache-Control: no-store`、Cookieの更新なし。作成要求記録・input_digestは変更せず、編集後の元の作成要求の再送も同じIDを返すだけとする。

## 競合・結果不明と再試行

新しい要求の版が古ければ409 update_conflict。入力を残して保存を止め、[編集画面](../画面仕様/コレクション本体の編集.md)で最新値を確認する。自動統合・強制上書きはしない。ONなら409 collection_not_stopped。同一要求番号のID/版/入力が異なれば409 update_request_conflictで、番号を勝手に交換して再送しない。

通信断・timeout・500・不正な成功応答は保存結果不明とし、本文・要求番号を保持して編集を止める。同じ内容で手動再試行する。最初の400/403だけは今回未保存と分かるため修正可能。結果不明後の再試行が400/403でも最初の保存済み可能性は消えず、元要求を維持する。認証済みで処理中の要求は後のログアウト・再設定でも完了し得る。

要求・下書きは画面メモリだけに保持する。離脱・再読み込み・認証状態変更で破棄した後は、詳細で現在値を確認する。新しい編集操作を旧要求の再試行とは扱わない。削除済みCollectionへの再送は404で、復活させない。APIの処理は10秒、画面の要求は15秒で打ち切る。

## エラー

| HTTP | error.code | 意味 |
|---|---|---|
| 400 | invalid_request | ID/要求番号/本文/版の形式、入力条件が不正 |
| 401 | unauthenticated | Cookie不正・欠落・重複、sessionなし |
| 403 | csrf_rejected | Origin欠落・不一致等 |
| 404 | not_found | 不存在、または他User所有 |
| 409 | update_conflict | 取得後の先行更新、または条件付きUPDATEの不成立 |
| 409 | collection_not_stopped | 新しい要求の対象がON |
| 409 | update_request_conflict | 同じ更新要求番号に対して異なるID/版/入力 |
| 500 | internal_error | DB処理・commit確認等の失敗。版の上限超過も全rollback |

構造化ログは固定項目のみ。入力・要求番号・Cookie・生DBエラーを返したり記録したりしない。入力条件の案内は画面の固定文言を使う。DB権限・保存構造は[DB設計](../database-design.md#collection本体の編集issue-74)。

## WatchTarget追加・編集への拡張（Issue #78）

本体と対象名・任意の補足指示を最後の保存で一括確定する方式、新規対象のONと既存対象の状態維持は合意済み。本節は上記の更新契約への追加を示す。

合意した拡張：既存PATCHへ対象変更を追加し、Collectionの版と更新要求番号を全体の保存で共有する。本体・対象・Collectionの版増加・要求記録を同じtransactionで確定する。既存の本人単位ロックと整合させ、同じ要求の再送で対象を重複作成しない。古い本体だけの要求形式とdigestを維持し、過去の成功要求の再試行も保持する。対象の省略を削除と解釈せず、情報源・有効状態・表示順を意図せず変更しない。

競合はCollection全体で検出し、対象変更後に古い本体編集を保存する場合も409とする方式に合意した。入力を保持し、最新値を確認して全体を編集し直す。対象変更だけを自動統合しない。以下のJSONと入力上限に合意し、子Issue #78で実装する。

### 対象変更の要求

既存の本体4項目・version・Idempotency-Keyに、任意のtargetsを追加する。targetsは全件置換でなく変更する行だけの配列とし、省略・空配列とも対象を変更しない。既存5項目の要求も引き続き受理する。

```json
{
  "name": "架空コレクション",
  "research_instruction": "",
  "initial_lookback_mode": "ai",
  "initial_lookback_days": null,
  "version": "3",
  "targets": [
    {"id": null, "name": "架空の新しい対象", "instruction": "新製品の発表を重視"},
    {"id": "01ARZ3NDEKTSV4RRFFQ69G5FAX", "name": "架空の既存対象", "instruction": "補足を変更"}
  ]
}
```

新規はid:null、既存は正規ULID。3項目を必須とし、null配列・未知/重複/欠落キー・重複する既存ID・型不正を拒否する。新規の有効状態と表示順はAPIが設定し、要求から受け取らない。既存IDのCollection所属を検証し、他CollectionのIDも存在を明かさず404とする。省略された対象の削除・変更は行わない。

入力条件：対象名は空白だけを拒否し1〜200 Unicodeコードポイント、補足指示は0〜10,000文字。既存本体と同様に空白・改行を保持し、NUL・不正Unicodeは拒否する。1要求の対象変更を最大20行、PATCH本文は1MiB以下とする。これはCollectionの登録総数上限ではない。画面は一度に1行追加し、保存前の新規行だけは取り消せる。保存済み対象の削除は追加しない。

初回／同一要求再送の成功は既存と同じ200、応答もid・versionを維持する。成功後は詳細GETで対象IDを含む現在値を読む。本体・対象・版増加・要求記録を同一transactionで保存し、変更行のどれかが不正なら全体を保存しない。再送は配列順を含む同じ要求本文を使い、既存の成功要求を変更しない。

## 情報源追加・編集への拡張（Issue #18）

代理判断により、本体・対象と情報源の一括保存を採用する。既存本体5項目と任意targetsに、独立した任意sources変更配列を追加する。省略・空配列は変更なし、nullは拒否。例：

```json
"sources": [
  {"id": null, "watch_target_id": "01ARZ3NDEKTSV4RRFFQ69G5FAX", "name": "架空の公式サイト", "url": "https://example.com/news", "instruction": "発表を重視"}
]
```

各行はこの5項目を必須とし、未知/重複/欠落キー・型不正を拒否する。idは新規null・既存正規ULID、watch_target_idは保存済み対象の正規ULID。情報源IDの重複を拒否し、情報源の対象間移動は認めない。対象が所有Collectionに所属すること、既存情報源が指定対象に所属することを確認し、不存在・他Collection・他User・別対象は同じ404。未送信情報源は維持する。新規対象と情報源の同時作成は扱わず、対象保存後に情報源を追加する。

表示名と補足指示は対象と同じUnicodeコードポイント・空白保持・NUL拒否条件で200/10,000文字。URLは[正本の入力条件](../product-definition.md#停止中collectionの情報源追加編集issue-18)に従い、Go net/urlの解析に加えて絶対HTTP/HTTPS・authority構文・ASCIIホスト・ポート・不正%・空白等を検証する。URLをtrim/正規化せず保存する。userinfoを構文で拒否するが、クエリやパスに埋め込まれた秘密の意味判定は行わない。画面では認証情報を含めない入力を案内する。名前の大文字小文字・末尾スラッシュ等も維持し、重複を許可する。ページ取得・DNS照会を行わない。

情報源は1要求20変更行、対象は従来の20変更行、本文は合計1MiB。sourcesをtargetsの後にomitemptyでdigest固定構造へ追加し、sourcesなしの本体/対象要求のdigestバイト列を維持する。配列順を含む同一本文・番号で再試行する。保存済み要求の照合は状態/版/配下所属の再更新より先に行い、後続更新後の旧成功要求は元結果を返すだけとする。

本人単位transaction advisory lockを共有し、本体条件付き更新・対象変更・情報源追加/編集・全体版1増加・要求記録を同一transactionで保存する。情報源編集だけでも全体版を増やし、旧本体/対象要求からの古い版保存は409とする。どの段階の失敗も全rollback。成功応答・認証/CSRF・所有者404・競合/結果不明/再試行契約は既存と同じ。履歴・snapshot・要求記録の過去digestは変更しない。

解析資料：[Go net/url](https://pkg.go.dev/net/url#Parse)、[URLのユーザー名](https://developer.mozilla.org/en-US/docs/Web/API/URL/username)。これらのパーサーを使うだけで入力条件やSSRFを保証するものではない。
