# API仕様

APIごとにMarkdownを置き、OpenAPIの `operationId`・`parameters`・`requestBody`・`responses` に相当する情報をまとめる。[OpenAPI公式仕様](https://spec.openapis.org/oas/v3.1.2.html)の項目を参考にするが、現段階では完全なOpenAPI YAMLやコード生成の入力ではない。

## API一覧

| API | 仕様書 | 合意状況 |
|---|---|---|
| `POST /api/login` | [ログイン](post-api-login.md) | パス・役割・ログイン後に本人情報を別取得する流れに合意。ステータス・JSON等の詳細は提案 |
| `GET /api/me` | [本人情報取得](get-api-me.md) | パス・役割・Cookieによる本人特定に合意。ステータス・JSON等の詳細は提案 |

対象は [Issue #53](https://github.com/smofktn/watchbrief/issues/53)。ログアウト等の後続APIは、そのIssueの設計時に追加する。

## 正本と採用状況

- 利用者向けの振る舞いは[プロダクト仕様](../product-definition.md)、通信・責務・処理順は[システム設計](../system-design.md#issue-53の基本設計)を参照する。
- APIごとの入力・出力・HTTPステータス・エラー・具体例は、このディレクトリで管理する。変更時は呼び出し側とシステム設計への影響を確認する。
- **合意済み**：利用者が了承した契約。実装・動作確認済みを意味しない。
- **提案**：相談中の具体案。記載・コミットだけで採用済みにはしない。YAML・JSON例も、この指定に従う。
- **未決定**：具体方式をまだ選んでいない。実装時に省略してよいという意味ではない。
- 旧AI案の実装と同じ項目でも、それだけを理由に今回の仕様へ採用しない。

## 共通の認証と秘密情報

**合意済み**：ID・パスワード認証とPostgreSQLのサーバー側セッションを使う。ブラウザへはランダムトークンをCookieで渡し、DBには照合用ハッシュを保存する。トークンはGoのcrypto/randで生成する32byte（256bit）の乱数とし、DBには元byte列のSHA-256だけを保存する。[生成・照合の設計](../system-design.md#認証トークンの生成照合合意済み)に従い、Cookie用の文字列表現と厳密な検証方法は実装前に決める。セッション記録IDのULIDは認証トークンとは別物。時間・無操作によるセッション失効は追加しない。

Cookieの保護属性とCSRFの基本方式は[次節](#cookieの保護とcsrf合意済み)のとおり合意済み。保存期間と更新も[次の合意](#cookieの保存更新合意済み)に従う。Cookie名・Path、ローカルHTTP時の扱い、送信元の具体的な設定・proxy接続は実装前に具体化する。本人情報をUser IDの入力で指定するAPIにはしない。

パスワード・Cookie・生トークン・パスワードハッシュ・接続設定をログや公開レスポンスへ含めない。認証トークンをJSONで返す方式や、VueがトークンをlocalStorageへ保存する方式は今回の提案に含めない。

## Cookieの保護とCSRF（合意済み）

Goが `Set-Cookie` で属性を指定し、ブラウザが保存・送信時に適用する。

| 属性 | 採用する設定と目的 |
|---|---|
| `HttpOnly` | 有効。Vue等のJavaScriptからトークンを読み取らせない。ブラウザによるAPIへのCookie送信は行われる |
| `Secure` | 本番HTTPS時に有効。ローカルHTTP開発時の扱いは別途具体化する |
| `SameSite` | `Lax` を明示。別サイトからのCookie送信を制限しつつ、通常のリンクからの訪問を妨げにくくする |
| `Domain` | 指定しない。Cookieを発行元ホストに限定し、別サブドメインへ共有しない |

CSRFはログイン前からGoのHTTP境界で検証する。Go標準の `net/http.CrossOriginProtection` を使い、さらにログイン等の変更要求は `Origin` と設定した画面の「プロトコル・ホスト・ポート」の完全一致を必須とする。`Origin` の欠落・`null`・不一致は拒否し、認証・セッション保存へ進めない。ログイン後の変更APIも同じ防御を使う方針であり、後続API自体を今回実装する決定ではない。

Go標準の防御は `Sec-Fetch-Site`・`Origin` 等を参照するが、両方がない要求を標準動作では許可する。そのため、変更要求に対する上記の `Origin` 必須チェックを省かない。GET等は標準防御で許可されるので、本人情報取得で新しいUser・セッションや業務データの変更を行わない。

この方式はブラウザからの意図しない操作を防ぐもので、認証の代わりではない。`Origin` を偽装できるブラウザ外のクライアントにも、有効なパスワード・認証トークンが必要である。`HttpOnly` も、XSS成立時のあらゆる操作を防ぐ保証ではない。Cookie属性だけにCSRF防御を任せない。

一次情報：[MDN Set-Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie)、[Go CrossOriginProtection](https://pkg.go.dev/net/http#CrossOriginProtection)、[OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html#fetch-metadata-headers)。Vite・将来のnginxを経由したときのHost・Originの扱いと、正規要求の成功・外部Origin等の拒否は実装時に検証する。現時点では未確認。

## Cookieの保存・更新（合意済み）

ブラウザ終了後も維持するため、保存期限付きCookieを使う。ログイン成功時に新しいトークンのCookieを設定し、本人情報APIでの認証成功時には同じトークンのCookieを返す。設定・更新時点から400日後へ保存期限を設定する（Max-Ageでは34,560,000秒）。DBセッションに期限・無操作失効は追加しない。Cookieの期限更新だけのためにトークンを交換したりセッションの行を増やしたりしない。未認証・内部障害の応答ではこの認証成功時の更新を行わない。

保存期限を指定しないCookieは、ブラウザのセッション終了時に失われ得る。Chromeは保存期限を設定時点から最大400日に制限し、期限内でもブラウザの設定や利用者の削除等でCookieは消え得る。400日間の保持や無期限ログインを保証する設計ではない。長期間使わずCookieが消えた場合は再ログインとなる。[MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie#expiresdate)、[Chromeの保存期限と更新](https://developer.chrome.com/blog/cookie-max-age-expires)。この制限と更新の仕組みを説明し、利用者と合意した。旧AI案と同じ数値であることを採用根拠にはしていない。

## 共通の応答案

[主な失敗時の画面表示](../product-definition.md#issue-53で最初に確認する画面)は合意済み。入力不正、認証失敗、本人情報取得時の未認証、通信失敗、内部障害を区別する。画面の文言と動作の正本はプロダクト仕様とし、ここへ重複して持たない。

以下の通信契約はすべて**提案**。HTTPステータス・コード名・JSON形式、および画面文言をAPIから返すか画面側で選ぶかは、まだ合意していない。下のJSON例は合意済みの内部障害向け文言を用いているが、それによってJSON形式まで採用済みになるものではない。

| 項目 | 提案 |
|---|---|
| JSON応答 | `Content-Type: application/json` |
| キャッシュ | 認証・本人情報の応答に `Cache-Control: no-store` |
| エラー形式 | 下記の `error.code` と `error.message` を持つJSON |
| メッセージ | 利用者向けの固定文言。SQL・内部例外・入力された秘密値を含めない |
| エラー判定 | 画面は `code` を判定に使い、`message` の文字列に依存しない |

```json
{
  "error": {
    "code": "internal_error",
    "message": "処理に失敗しました。時間をおいて再度お試しください。"
  }
}
```

| JSONフィールド | 型 | 必須案 | 説明 |
|---|---|---|---|
| `error` | object | 必須 | エラー情報 |
| `error.code` | string | 必須 | APIごとの表に示す識別用コード |
| `error.message` | string | 必須 | 表示可能な固定メッセージ |

通信切断ではHTTPレスポンス自体を受け取れない場合がある。画面は認証拒否と通信・内部障害を区別し、通信失敗だけで未ログインやログイン成功を断定しない。

## 更新・確認方法

各APIには、採用状況、リクエスト、レスポンス、処理順、副作用、失敗時の扱い、未決定事項、確認観点を記載する。相談で合意した項目だけ採用状況を更新し、対象Issueへ理由を記録する。

このディレクトリは初回の設計資料として終わらせず、継続して保守する通信契約とする。APIの追加・変更時は、対応する仕様書と実装・テストを同じ変更で更新し、[画面仕様](../画面仕様/README.md)の入力・表示・状態との対応も確認する。画面の配置・操作は画面仕様に置き、API仕様へ重複して持たない。仕様書には現在の契約・未決定事項・確認観点を残し、時点に依存する検証結果と未確認の範囲はIssue・PRへ記録する。

現在は設計文書のみであり、Swagger UI表示、OpenAPIの機械的検証、API実装との一致、実HTTP通信の検証は未実施。完全なOpenAPI定義や生成ツールの採用は別途判断する。
