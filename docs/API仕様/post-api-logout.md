# POST /api/logout — 対象セッションのログアウト

## 採用状況と範囲

[Issue #54](https://github.com/smofktn/watchbrief/issues/54#issuecomment-6687899204)で以下のAPI契約に合意した。利用者向けの正本は[プロダクト仕様](../product-definition.md#issue-54のログアウト)、処理分担は[システム設計](../system-design.md#issue-54のログアウトapi)。実装・動作確認の結果はIssueに記録する。画面からの呼び出しと失敗時の再試行は[画面仕様](../画面仕様/ログイン・本人情報表示.md#ログアウトissue-54)に従う。

## リクエスト（合意済み）

- `POST /api/logout`。本文・パス／クエリパラメーターは不要。本文・クエリを対象指定に使わず、Content-Typeも要求しない。
- ブラウザが送信する `watchbrief_session` Cookieだけで対象を決める。User ID・セッション記録ID・トークンをJSONやURLで受け取らない。
- Cookieは[本人情報APIと同じ厳密検証](get-api-me.md#未決定確認観点)を適用する。ヘッダー全行から同名Cookieを数え、欠落・不正・重複はDB操作前に401。
- [共通CSRF防御](README.md#cookieの保護とcsrf合意済み)をログインと共通のHTTP受付で適用する。Originは1件・APP_ORIGIN完全一致必須で、Go標準CrossOriginProtectionも検証する。CSRF拒否はCookie検証・DB操作より先。
- GET・HEAD等ではログアウトしない。POST以外は405で、DB・Cookieを変更しない。

## レスポンス（合意済み）

| ステータス | 本文 | CookieとDBの扱い |
|---|---|---|
| 204 | なし | トークンのハッシュに対応するDBセッションを物理削除した後にCookieを削除する。対象行が既にない場合も成功 |
| 401 | 共通JSON、`unauthenticated` | Cookie欠落・不正・重複。DB・Cookieを変更しない |
| 403 | 共通JSON、`csrf_rejected` | Origin検証等の失敗。DB・Cookieを変更しない |
| 500 | 共通JSON、`internal_error` | DB等の内部障害。成功とせずCookieを残し、同じCookieで再試行可能 |

成功・上記JSONエラーは `Cache-Control: no-store`。エラーは[共通形式](README.md#共通の応答案)で、既存の固定文言を使う。トークン・ハッシュ・DBエラー詳細を返さず、ログにも出さない。

Cookie削除は同じ名前・Path=/・Domain未指定で空値を設定し、GoのMaxAge=-1（HTTP上はMax-Age=0）と過去のExpiresを指定する。HttpOnly・SameSite=Laxと、HTTPS時のSecureは発行時と同じ条件を使う。

## 処理・再試行

1. HTTP受付でCSRF・Cookieを検証し、元32byteのSHA-256を計算する。
2. Logoutユースケースを通して、sqlcの `DELETE FROM app.auth_sessions WHERE token_hash = $1` を実行する。1文の確定を待ち、削除件数0も成功とする。
3. 成功後だけCookie削除・204を返す。DB処理の失敗／中断は500とし、Cookie削除を送らない。

Cookieをブラウザへ届ける操作とDB削除は原子的にできない。削除後に応答が失われた場合、またはDBの確定結果が通信断で不明になった場合も、正規形式の同じトークンで再実行すると204になる。削除後の旧トークンで `GET /api/me` を呼ぶと401になる。Cookieそのものがない要求は401であり、この再試行契約とは区別する。

同じUserの他端末や他Userのセッションは削除しない。DB用ロールのDELETE権限と、アプリで削除対象を絞る責務は[DB設計](../database-design.md#issue-54のログアウト保存処理)を参照する。時間失効・全端末管理・パスワード再設定は追加しない。

処理timeoutは既存APIと同じ10秒。タイムアウトは認証セッションの時間失効ではない。ログアウト確定前に既に認証を通過した並行要求の応答を取り消す仕組みは追加しない。確定後の新しい認証要求は旧トークンを拒否する。

一次情報：[PostgreSQL DELETE](https://www.postgresql.org/docs/18/sql-delete.html)、[Go Cookie](https://pkg.go.dev/net/http#Cookie)、[Go CrossOriginProtection](https://pkg.go.dev/net/http#CrossOriginProtection)、[MDN Set-Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie)。
