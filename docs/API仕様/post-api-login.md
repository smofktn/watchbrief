# POST /api/login — ログイン

## 概要・採用状況

| 項目 | 内容 | 状態 |
|---|---|---|
| メソッド・パス | `POST /api/login` | 合意済み |
| 目的 | ログインID・パスワードで認証し、成功時にセッションCookieを発行する | 合意済み |
| 後続の呼び出し | Cookie保存後に [`GET /api/me`](get-api-me.md) で本人情報を取得する | 合意済み |
| 既存の認証 | 不要。未ログインの利用者が呼ぶ | 合意済みの役割からの前提 |
| `operationId` | `login` | 提案 |
| 正常ステータス | `204 No Content`、本文なしでCookie発行 | 合意済み |
| 入力JSON | 必須の文字列 `login_id`・`password` を本文に送る | 合意済み |
| ヘッダー・エラー詳細 | 下記の定義 | 提案・一部未決定 |

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

次のJSON形式は**合意済み**。必須の文字列 `login_id`・`password` をリクエスト本文で送り、秘密値をURL・クエリへ含めない。不明フィールド等の解析の詳細は未決定。

| フィールド | 型 | 必須 | 制約・扱い |
|---|---|---|---|
| `login_id` | string | 必須 | 英大文字を小文字へ正規化し、`a-z`・数字・`_`・`-`、1〜64文字を検証する。ID条件自体は既存合意 |
| `password` | string | 必須 | Argon2idの共通処理で照合する。NFC後15〜128コードポイント、文字種混在不要、日本語・スペース等も許可し、前後の空白を削らず大文字・小文字を区別する。入力条件は[プロダクト仕様](../product-definition.md#ユーザーと認証)に合意済み。具体プロファイルは[共通ハッシュ](../password-hashing.md)を参照 |

```json
{
  "login_id": "fictional_user",
  "password": "example-only-not-a-real-password"
}
```

例は架空値であり、利用するパスワードを文書・Issue・ログへ転記しない。

## レスポンス

### 正常系：204 No Content（合意済み）

本文を返さず、DBへセッションを保存できた場合だけ `Set-Cookie` を返す。Cookieの保護属性は[共通仕様](README.md#cookieの保護とcsrf合意済み)に従う。保存更新も[共通仕様](README.md#cookieの保存更新合意済み)に従う。名前はwatchbrief_session、Pathは `/`。ローカルHTTPと文字列表現は共通仕様に従う。`204` でもヘッダーでCookieを渡し、JSONにトークンや本人情報を返さない。[共通の応答案](README.md#共通の応答案)のキャッシュ抑止を適用する。

ブラウザはCookieを保存し、画面は本人情報APIを呼ぶ。その取得に成功するまでログインIDの表示を完了しない。

### エラー応答（合意済みと提案）

本文は[共通エラー形式](README.md#共通の応答案)。400・401・403・500のステータスとコード・分類は合意済み。413・415および上限値、評価順や複数の不備が重なった場合の優先順位は提案・未決定。保存・Cookieの扱いは既存の認証・整合性の合意を維持する。

| ステータス | `error.code` | 状況 | 保存・Cookieの扱い案 |
|---|---|---|---|
| `400` | `invalid_request` | JSON形式・必須項目・型・入力条件が不正 | 認証・保存へ進めず、発行しない |
| `401` | `invalid_credentials` | 未知のID、またはパスワード不一致 | セッションを作らず、発行しない。2つの理由を画面で区別しない |
| `403` | `csrf_rejected` | CSRF検証に失敗 | 認証・保存へ進めず、発行しない |
| `413` | `request_too_large` | リクエストサイズ超過 | 認証・保存へ進めず、発行しない。上限値は未決定 |
| `415` | `unsupported_media_type` | 対応外のContent-Type | 認証・保存へ進めず、発行しない |
| `500` | `internal_error` | DB・乱数生成・ハッシュ処理等の内部障害 | DB保存失敗時は成功Cookieを返さない。内部原因は公開しない |

ログイン試行制限の具体実装と、それに伴う `429` 応答は今回決めず、#49の範囲を維持する。

## OpenAPI風の定義（抜粋・提案）

以下は `paths` に相当するYAMLの抜粋。`x-nfc-min-code-points`・`x-nfc-max-code-points` はNFC後の条件を示す独自注記であり、標準JSON Schemaの長さ制約ではない。`x-design-status` は設計上の注記。Cookie・CSRFの合意済み条件は共通仕様に従う。413・415等には未採用の提案が残るため、この抜粋だけを完成した実装契約として扱わない。

```yaml
paths:
  /api/login:
    post:
      operationId: login
      summary: ID・パスワードでログインする
      tags: [Authentication]
      x-design-status: partially-agreed
      description: 既存ログインは不要。Go標準CSRF防御とOrigin完全一致検証をログイン前から適用する。
      parameters:
        - name: Origin
          in: header
          required: true
          description: 設定した画面のプロトコル・ホスト・ポートと完全一致。欠落・nullは拒否する。
          schema:
            type: string
      requestBody:
        x-design-status: agreed-fields
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
                  x-nfc-min-code-points: 15
                  x-nfc-max-code-points: 128
                  description: NFCでそろえた後のUnicodeコードポイントで数える。JSON原文のminLength/maxLengthでは表現しない。前後の空白を削らず、大文字・小文字を区別。Argon2idの具体プロファイルは共通ハッシュ仕様を参照
      responses:
        '204':
          x-design-status: agreed
          description: セッション保存成功。本文なし。Cookie保存後に本人情報を別取得する
          headers:
            Set-Cookie:
              description: watchbrief_session。Path=/、パディングなしBase64URL。保護属性と400日の保存期限は共通仕様
              schema:
                type: string
        '400':
          description: 入力不正。本文は共通エラー形式
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

Cookieの具体設定は共通仕様、Argon2id・NFCは共通ハッシュ仕様を参照する。最終的な本文上限・413/415応答、入力不正の詳細文言・評価順、本番proxy・TLSでの挙動は今後具体化・検証する。

正常ログイン、誤パスワード・未知ID、入力不正、CSRF拒否、DB保存失敗時のCookie非発行、ログ・レスポンスへの秘密情報非混入を検証する。実装の検証結果と未確認範囲はIssueへ記録する。

## 現在の実装範囲

現在の[Backend](../../backend/README.md)は、Origin検証後にJSON・入力条件を確認し、User取得・共通Argon2id照合・セッションINSERTを行う。保存成功時だけ204とCookieを返す。未知IDでもダミーハッシュを照合し、誤パスワードと同じ401にする。DB等の内部障害は秘密値を含まない500と固定項目の構造化ログにする。

4KiB上限・厳密なJSON項目検証・Content-Type不正時の400等は検証用の仮実装。未知・重複項目、null、末尾の別JSON、不正UTF-8・対にならないUTF-16サロゲートを拒否する。413/415等の提案を採用済みに更新するものではない。操作の10秒timeoutはDB処理等を対象とし、セッション失効ではない。

FrontendからVite経由でログインし、204の後は本人情報APIを呼んでログインIDを表示する。取得失敗時には成功表示を確定せず、未認証・通信・内部障害を区別する。
