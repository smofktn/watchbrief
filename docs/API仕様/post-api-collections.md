# Collection本体の新規作成（Issue #18）

## 採用状況と範囲

Collection本体から新規保存を通す範囲と、同じ要求の再試行で重複作成しない方式に合意した。以下のJSON、要求番号の形式、入力上限、応答・保存構造は、その範囲を具体化する実装判断である。配下設定の編集、有効化、削除、並べ替えは後続範囲とする。

`POST /api/collections` は停止中Collectionを作り、既存の[本人限定一覧・詳細](get-api-collections.md)へ接続する。所有者はセッションから決める。Cookie・CSRF・秘密情報の扱いは[共通API仕様](README.md)に従う。

## 要求

`Content-Type: application/json` と、1つの `Idempotency-Key` ヘッダーを必須とする。要求番号はブラウザの[crypto.randomUUID()](https://developer.mozilla.org/en-US/docs/Web/API/Crypto/randomUUID)による小文字UUID v4で、同じ入力の再試行では同じ値を使う。これは作成操作の識別値で、エンティティIDや認証情報ではない。保存するCollectionのIDはAPI側でULIDを生成する。

本文は次の4項目を必須とする。「任意」の調査方針は空文字で送る。

```json
{
  "name": "架空のコレクション",
  "research_instruction": "",
  "initial_lookback_mode": "ai",
  "initial_lookback_days": null
}
```

| 項目 | 条件 |
|---|---|
| name | 空白だけにせず1〜200 Unicodeコードポイント。前後空白を勝手に変更しない |
| research_instruction | 0〜10,000 Unicodeコードポイント。改行・空白を保持する |
| initial_lookback_mode | ai または days |
| initial_lookback_days | aiではnull、daysでは1〜365の整数 |

名前・調査方針の上限は、通常の入力に余裕を持たせつつ要求サイズを制限するための実装判断。本文全体は64KiBまで。不正UTF-8・不対サロゲート、文字列のNUL、未知・重複項目、欠落、型不正、末尾の別JSONを拒否する。user_id、enabled、display_order、配下設定は受け取らない。空白だけの名前は拒否するが、名前の重複自体は許可する。

## 保存と成功応答

CSRF、Cookieの形式、本文の構文を確認した後、セッションから所有者を特定し、入力の業務条件を検証する。認証を通過済みの進行中要求は、後のログアウト・再設定後も保存を完了し得る。画面の消去・応答破棄でDBのcommitまで取り消せるとは扱わない。

本人の作成処理をDBトランザクションのadvisory lockで直列化し、要求番号の記録を調べる。同じ番号・同じ入力なら既存IDを返す。初回はCollectionと要求記録を同じトランザクションで保存する。入力一致は、検証後の4項目を固定順JSONへ直列化したSHA-256で確認する。

Collectionはenabled=false、配下設定なしで保存する。本人の一覧末尾へ加え、既存行の表示順は変更しない。列権限と具体保存方式は[DB設計](../database-design.md#collection本体の新規保存issue-18)を参照する。

| 結果 | HTTP | 本文 |
|---|---|---|
| 初回保存 | 201 | `{"id":"正規ULID"}` |
| 同じ要求の再送 | 200 | 初回と同じID |

両方とも `Location: /api/collections/{id}`、`Cache-Control: no-store`、JSONを返す。Cookieは更新しない。画面はIDの形式を確認し、既存の詳細取得へ進む。

## 保存結果が不明な場合（合意済み）

DB保存が成功しても応答だけが途切れることがある。画面は同じ要求番号・同じ本文で再試行する。名前の一致で重複判定しないので、別の新規操作で同名Collectionを作ることはできる。同時要求でも本人と要求番号の組は1件になる。

結果不明中は入力を保持して編集を止め、同じ内容の手動再試行を許可する。自動再試行はしない。最初の要求が400/403で保存前に拒否されたと確認できた場合は編集可能に戻す。結果不明を一度経た後は、再試行が400/403でも最初の保存結果は確定しないため、要求番号・本文を維持する。

要求番号は画面メモリだけに保持し、画面離脱・再読み込み・認証状態変更・ログアウトで破棄する。この後は一覧で保存結果を確認する。同じ要求番号を失った後の新規作成まで重複排除するものではない。下書きや番号をlocalStorage/sessionStorageへ保存しない。

Collection削除後も要求記録を残し、古い要求の再送で設定を復活させない。削除済み、または同じ番号で異なる入力を送った場合は409とする。保存済みの設定全文・要求記録を応答へ含めない。

## エラー

共通のerror.code/error.message形式を使い、応答はno-store。内部原因・入力値・要求番号・認証情報をログへ含めない。

| HTTP | error.code | 意味 |
|---|---|---|
| 400 | invalid_request | ヘッダー・本文の形式、入力条件が不正。保存しない |
| 401 | unauthenticated | Cookie不正・重複・欠落、セッションなし |
| 403 | csrf_rejected | Origin欠落・不一致等。保存しない |
| 409 | creation_conflict | 同じ要求番号の入力不一致、または作成先が削除済み |
| 500 | internal_error | 認証照合・乱数・DB処理等の失敗。画面は保存結果不明として扱う |

業務入力エラーの400には任意のerror.fieldsとして、name・research_instruction・initial_lookback_daysの固定案内を含める。画面は任意のAPIメッセージをそのまま表示せず、ローカル検証と固定案内を使う。

APIの処理は10秒、画面の要求は15秒で打ち切る。500にはcommitの応答を確認できなかった場合も含まれるため、必ず未保存とは断定しない。409では一覧確認へ案内し、新しい要求番号で自動再送しない。

## 確認観点

新規→詳細→一覧→再読み込み、初回期間の両方式、入力境界、本人所有、CSRF、同時再送の1件化、異なる要求番号による同名作成、異なるUserによる同一番号、2回目の書込失敗によるrollback、保存後の応答喪失、削除後再送、最小権限を確認する。画面の入力保持・消去は[新規作成の画面仕様](../画面仕様/コレクションの新規作成.md)を参照する。実行結果と未確認範囲はIssue・PRへ記録する。

方式の根拠：[RFC 9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2)、[PostgreSQLの一意制約](https://www.postgresql.org/docs/18/ddl-constraints.html#DDL-CONSTRAINTS-UNIQUE-CONSTRAINTS)、[トランザクション分離](https://www.postgresql.org/docs/18/transaction-iso.html)、[advisory lock](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS)。外部サービスは追加しない。
