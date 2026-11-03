# POST /api/login — ログイン

## 概要・採用状況

| 項目 | 内容 | 状態 |
|---|---|---|
| メソッド・パス | `POST /api/login` | 合意済み |
| 目的 | ログインID・パスワードで認証し、成功時にセッションCookieを発行する | 合意済み |
| 後続の呼び出し | Cookie保存後に [`GET /api/me`](get-api-me.md) で本人情報を取得する | 合意済み |
| 既存の認証 | 不要。未ログインの利用者が呼ぶ | 合意済みの役割からの前提 |
| `operationId` | `login` | 提案 |
| 正常ステータス | `204 No Content` | 提案 |
| JSON・ヘッダー・エラー詳細 | 下記の定義 | 提案・一部未決定 |

[共通事項](README.md)と[基本設計](../system-design.md#issue-53の基本設計)も参照する。

## リクエスト

### Parameters・認証

パスパラメーターとクエリパラメーターは使わない案。既存のセッションCookieを認証成功の根拠として再利用せず、ID・パスワードを照合する。

| 入力 | 場所 | 条件 | 状態 |
|---|---|---|---|
| `Content-Type` | header | `application/json` | 提案 |
| `Origin` | header | 設定した画面のOriginと完全一致を必須とし、欠落・null・不一致を拒否する | 合意済み |
| `Sec-Fetch-Site` 等 | header | Go標準のCrossOriginProtectionでも検証する。ブラウザが付与する | 標準防御の採用は合意済み |

### Request body

次のJSON形式は**提案**。秘密値をURL・クエリへ含めない。

| フィールド案 | 型 | 必須案 | 制約・扱い |
|---|---|---|---|
| `login_id` | string | 必須 | 英大文字を小文字へ正規化し、`a-z`・数字・`_`・`-`、1〜64文字を検証する。ID条件自体は既存合意 |
| `password` | string | 必須 | Argon2idの共通処理で照合する。15〜128文字、文字種混在不要、日本語・スペース等も許可し、前後の空白を削らず大文字・小文字を区別する。入力条件は[プロダクト仕様](../product-definition.md#ユーザーと認証)に合意済み。具体パラメーター等は未決定 |

```json
{
  "login_id": "fictional_user",
  "password": "example-only-not-a-real-password"
}
```

例は架空値であり、利用するパスワードを文書・Issue・ログへ転記しない。

## レスポンス

### 正常系案：204 No Content

本文を返さず、DBへセッションを保存できた場合だけ `Set-Cookie` を返す。Cookieの保護属性は[共通仕様](README.md#cookieの保護とcsrf合意済み)に従う。保存更新も[共通仕様](README.md#cookieの保存更新合意済み)に従う。名前・Path、ローカルHTTP時の扱いは未決定。`204` でもヘッダーでCookieを渡す案であり、JSONにトークンや本人情報を返さない。[共通の応答案](README.md#共通の応答案)のキャッシュ抑止を適用する。

ブラウザはCookieを保存し、画面は本人情報APIを呼ぶ。その取得に成功するまでログインIDの表示を完了しない。

### エラー応答案

本文は[共通エラー形式案](README.md#共通の応答案)。下記のステータス・コード・分類は提案であり、評価順や複数の不備が重なった場合の優先順位は実装前に整理する。

| ステータス案 | `error.code` 案 | 状況 | 保存・Cookieの扱い案 |
|---|---|---|---|
| `400` | `invalid_request` | JSON形式・必須項目・型・入力条件が不正 | 認証・保存へ進めず、発行しない |
| `401` | `invalid_credentials` | 未知のID、またはパスワード不一致 | セッションを作らず、発行しない。2つの理由を画面で区別しない |
| `403` | `csrf_rejected` | CSRF検証に失敗 | 認証・保存へ進めず、発行しない |
| `413` | `request_too_large` | リクエストサイズ超過 | 認証・保存へ進めず、発行しない。上限値は未決定 |
| `415` | `unsupported_media_type` | 対応外のContent-Type | 認証・保存へ進めず、発行しない |
| `500` | `internal_error` | DB・乱数生成・ハッシュ処理等の内部障害 | DB保存失敗時は成功Cookieを返さない。内部原因は公開しない |

ログイン試行制限の具体実装と、それに伴う `429` 応答は今回決めず、#49の範囲を維持する。

## OpenAPI風の定義（抜粋・提案）

以下は `paths` に相当するYAMLの抜粋。`x-design-status` は設計上の注記。Cookie・CSRFの合意済み条件は共通仕様に従う。Cookie名・Path・ローカルHTTP時の扱い等が未決定のため、この抜粋だけを完成した認証仕様として扱わない。

```yaml
paths:
  /api/login:
    post:
      operationId: login
      summary: ID・パスワードでログインする
      tags: [Authentication]
      x-design-status: proposed-details
      description: 既存ログインは不要。Go標準CSRF防御とOrigin完全一致検証をログイン前から適用する。
      parameters:
        - name: Origin
          in: header
          required: true
          description: 設定した画面のプロトコル・ホスト・ポートと完全一致。欠落・nullは拒否する。
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [login_id, password]
              properties:
                login_id:
                  type: string
                  minLength: 1
                  maxLength: 64
                  pattern: '^[A-Za-z0-9_-]+$'
                  description: 英大文字を小文字へ正規化して照合する
                password:
                  type: string
                  writeOnly: true
                  minLength: 15
                  maxLength: 128
                  description: Unicodeコードポイントで数える。前後の空白を削らず、大文字・小文字を区別。Argon2idの具体パラメーター等は未決定
      responses:
        '204':
          description: セッション保存成功。本文なし。Cookie保存後に本人情報を別取得する
          headers:
            Set-Cookie:
              description: 認証用Cookie。保護属性と400日の保存期限は共通仕様。名前・Pathは未決定
              schema:
                type: string
        '400':
          description: 入力不正。本文は共通エラー形式案
        '401':
          description: ID・パスワード認証失敗
        '403':
          description: CSRF検証失敗
        '413':
          description: リクエストサイズ超過
        '415':
          description: 対応外のContent-Type
        '500':
          description: 内部障害。秘密情報・内部原因を公開しない
```

## 処理順・副作用

1. HTTP受付でリクエスト形式・サイズ・CSRFを確認する（詳細な評価順は未決定）。
2. ユースケースでUserを取得し、共通ハッシュ処理でパスワードを照合する。
3. crypto/randで新しい32byteのランダムトークンと、別のセッション記録ID（ULID）を生成する。
4. 元のトークンbyte列のSHA-256とUserの対応をPostgreSQLへ保存する。生トークンは保存しない。
5. 保存成功後にHTTP受付からCookieを発行する。

成功時はセッションの行が増える。既存セッションの削除・無効化は今回の合意に追加しない。DB保存直後の通信断では、Cookieを取得できないセッションが残り得る。画面はログインを自動再送せず、通信失敗を認証拒否と区別する案とする。

## 未決定・確認観点

Cookie名・PathとローカルHTTP時の扱い、Origin設定とproxy接続、共通ハッシュの具体パラメーター等・Unicode処理の詳細、JSON不明フィールドの扱い、上限値、エラー形式と優先順位、HTTPステータスは実装前に具体化する。Cookie・CSRFの合意済み条件は共通仕様に従う。

正常ログイン、誤パスワード・未知ID、入力不正、CSRF拒否、DB保存失敗時のCookie非発行、ログ・レスポンスへの秘密情報非混入を検証する。現時点では仕様案の整理のみで、実HTTP通信は未確認。
