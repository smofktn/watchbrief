# GET /api/me — 本人情報取得

## 概要・採用状況

| 項目 | 内容 | 状態 |
|---|---|---|
| メソッド・パス | `GET /api/me` | 合意済み |
| 目的 | Cookieで本人を特定し、自分のログインIDを返す | 合意済み |
| 呼び出すタイミング | ログイン後と、画面の初期表示・再読込時 | 合意済み |
| 認証 | PostgreSQLのサーバー側セッションとCookie | 合意済み、Cookie名等は未決定 |
| `operationId` | `getCurrentUser` | 提案 |
| 正常ステータス・JSON | `200` と `{"login_id":"fictional_user"}` | 提案 |

[共通事項](README.md)、[ログインAPI](post-api-login.md)、[基本設計](../system-design.md#issue-53の基本設計)も参照する。

## リクエスト

### Parameters・認証

| 入力 | 場所 | 条件 | 状態 |
|---|---|---|---|
| 認証トークン | Cookie | 32byte乱数のトークンをブラウザがCookieとして自動送信する。Cookie名・文字列表現は未決定 | Cookieで本人を特定することは合意済み |
| User ID | 受け取らない | 取得対象はCookieで特定し、画面から指定しない | 基本設計に従う |

パス・クエリパラメーターとリクエスト本文は使わない案。認証トークンをURL・クエリ・JSONへ入れず、VueがCookieを読み取って付け直す設計にはしない。

## レスポンス

### 正常系案：200 OK

```json
{
  "login_id": "fictional_user"
}
```

| フィールド案 | 型 | 必須案 | 内容 |
|---|---|---|---|
| `login_id` | string | 必須 | Cookieで認証したUserの、DBに保存された小文字のログインID |

Cookieのトークン、パスワードハッシュ、内部セッションID等は返さない。[共通の応答案](README.md#共通の応答案)のJSONヘッダー・キャッシュ抑止を適用する。[共通仕様](README.md#cookieの保存更新合意済み)に従い、認証成功時に同じトークンのCookieの保存期限を更新する。

### エラー応答案

本文は[共通エラー形式案](README.md#共通の応答案)。ステータス・コード・Cookieの不正形式や重複の判定詳細は提案として扱う。

| ステータス案 | `error.code` 案 | 状況 | 画面の動きの案 |
|---|---|---|---|
| `401` | `unauthenticated` | Cookieなし、不正なCookie、対応するセッションがない等 | 本人情報を表示せず、ログインフォームを表示する |
| `500` | `internal_error` | DB接続・検索等の内部障害 | 認証状態を断定せず、通信・サーバーエラーと再試行手段を表示する |

存在しないUserを画面から指定するAPIではないため、他UserのID指定による `404` の仕様をこのAPIには追加しない。

## OpenAPI風の定義（抜粋・提案）

以下は `paths`・`components` に相当するYAMLの抜粋。`SESSION_COOKIE_NAME_TBD` は未決定のCookie名を表す仮の名前であり、実際の採用名ではない。認証トークンをOpenAPIのBearer認証として表現しない。

```yaml
paths:
  /api/me:
    get:
      operationId: getCurrentUser
      summary: Cookieで認証した本人のログインIDを取得する
      tags: [Authentication]
      x-design-status: proposed-details
      security:
        - sessionCookie: []
      responses:
        '200':
          description: 本人情報
          content:
            application/json:
              schema:
                type: object
                required: [login_id]
                properties:
                  login_id:
                    type: string
                    minLength: 1
                    maxLength: 64
                    pattern: '^[a-z0-9_-]+$'
              example:
                login_id: fictional_user
        '401':
          description: 未認証。本文は共通エラー形式案
        '500':
          description: 内部障害。未認証と区別する
components:
  securitySchemes:
    sessionCookie:
      type: apiKey
      in: cookie
      name: SESSION_COOKIE_NAME_TBD
      description: ランダムトークンを保持するCookie。保護属性と認証成功時の400日の保存期限更新は共通仕様。名前・Pathは未決定
```

## 処理順・副作用

1. HTTP受付がCookieを読み取り、ユースケースへ渡す。
2. トークン文字列をデコード・検証し、元の32byte乱数のSHA-256からセッションを検索してUserを特定する。DBアクセス・ハッシュ処理はユースケースのインターフェースを通して行う。
3. HTTP受付が公開するログインIDだけをJSONに変換する。

DBに新しいUser・セッションを作らず、パスワードを再要求しない。時間・無操作によるセッション失効や最終アクセス記録を追加しない。認証成功時は同じトークンのCookieをSet-Cookieで返し、保存期限をその時点から400日後へ更新する。DBのセッション行を増やしたり、期限・最終アクセス日時を更新したりしない。

## 未決定・確認観点

Cookieの保護属性は[共通仕様](README.md#cookieの保護とcsrf合意済み)に従う。Cookie名・Path、ローカルHTTP時の扱い、トークンの文字列表現・厳密な検証方法、エラー形式、Cookie不正時の応答詳細、処理上限等は実装前に具体化する。

ログイン後のCookieによる本人情報取得、Cookieなし・不正・該当セッションなしの拒否、DB障害と未認証の区別、APIの実プロセス再起動後の認証維持、秘密情報非混入を検証する。Cookieの具体的な保存方式を決めた後、ブラウザ再起動時の挙動も確認する。現時点では仕様案の整理のみで、実HTTP通信は未確認。
