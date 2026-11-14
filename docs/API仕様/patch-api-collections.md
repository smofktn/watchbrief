# Collection本体の更新（Issue #74）

## 採用状況と範囲

版番号と更新要求番号で、先行更新との競合と応答喪失後の再試行を区別する方式に合意した。`PATCH /api/collections/{ULID}` は停止中Collectionの名前・任意の調査方針・初回確認期間だけを一体として変更する。以下の厳密JSON・版の物理型・エラーコード・保存構造は、合意を具体化する実装判断である。

認証・CSRFは[共通API仕様](README.md)に従う。所有者はセッションから決め、他Userと不存在は本文も同じ404とする。配下CRUD・有効化・並べ替えは今回追加しない。

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

5項目を必須とする。本体4項目の値・文字数・Unicode・空白・NUL・日数の扱いは[作成APIの入力条件](post-api-collections.md#要求)を維持する。任意方針は空文字、AI方式の日数はnull。versionは取得済みの編集元の版で、1〜9223372036854775807の正規十進文字列（符号・先頭0なし）。JSON数値は拒否し、ブラウザの整数精度に依存させない。

64KiBの本文上限、不正UTF-8・不対サロゲート・未知/重複/欠落項目・型不正・末尾の別JSONを拒否する。enabled・display_order・所有者・配下を受け取らない。

## 保存と成功応答

既存のCSRF・Cookie形式・厳密JSON検査の後、ユースケースがセッション由来の所有者と入力を検証する。DBは本人の作成処理と共有するtransaction advisory lockを取り、所有Collectionの存在、更新要求記録、停止状態、版を調べる。同じ所有者の別Collectionに対する同一要求番号の競合も直列化する。

初回は所有者・ID・版一致・停止中を条件に本体4列、版、updated_atだけをUPDATEし、版を1増やす。更新結果と要求記録を同じtransactionで確定し、片側だけ残さない。既存本体、配下設定、履歴を単一SQLで読む取得契約は維持する。将来の別の更新処理も版の増加を守る必要がある。

初回と同じ要求番号・同じ本文なら、元の保存結果を200で返す。入力一致はID・編集元版・本体4項目を固定構造のJSONへ直列化したSHA-256で判定する。再送では版やupdated_atを更新しない。

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
