# Issue #53：ログインの実装方式

> **Issue #53の既存案について**：以前のAIが作成した未完成の実装案と、それに伴う文書変更を参考として保持している。具体バージョン、ハッシュ、Cookie・CSRF、migration等は、今回利用者と確認したもの以外は採用済みと扱わない。過去の検証結果も今回の実動作確認とは区別する。現在の進め方は[作業計画](work-plan.md#利用者主体で子issueを進める運用合意済み)を参照する。

既存のID・パスワード認証、PostgreSQLのサーバー側セッション、ULIDと認証トークンの分離を具体化する。今回の実装判断であり、Issue #1の時点で以下の詳細まで採用済みだったという意味ではない。利用者向けの仕様変更は行わない。ログアウト、元URL復帰、設定・Report取得、パスワード再設定は #54以降で扱う。

## 最小構成と選定

| 対象 | 今回の選択 | 比較・理由と一次情報 |
|---|---|---|
| API | Go 1.27.3、標準 `net/http` | HTTP frameworkを増やさず、メソッド別routing・サイズ制限・timeoutを明示する。[Goリリース](https://go.dev/doc/devel/release)、[net/http](https://pkg.go.dev/net/http) |
| DB | PostgreSQL 18.6をローカル検証対象とする | サポート中の18系を使い、19 RCの採用を避ける。AWSでの具体配備版は #21で再確認。[サポート表](https://www.postgresql.org/support/versioning/) |
| DBアクセス | SQL＋sqlc 1.31.1、pgx/v5 5.11.0 | SQL＋sqlcは #53での指示に従う。GORMを追加せずSQL・制約を直接レビューする。`database/sql` adapterと比較し、PostgreSQL専用のpgx interfaceを選び、sqlcの公式対応を使う。[sqlc pgx](https://docs.sqlc.dev/en/stable/guides/using-go-and-pgx.html)、[sqlcリリース](https://github.com/sqlc-dev/sqlc/releases/tag/v1.31.1)、[pgx](https://pkg.go.dev/github.com/jackc/pgx/v5) |
| migration | 番号付きup/down SQL＋psqlによる明示適用 | ORM自動migrationやGo起動時のDDLを使わない。最初の1版は `migrate.sql` がtransaction・advisory lock・version記録を担当する。別ツール導入の依存を増やさない。将来migration数が増えたら専用ツールを比較する。[psql](https://www.postgresql.org/docs/18/app-psql.html)、[transaction](https://www.postgresql.org/docs/18/tutorial-transactions.html) |
| UI | Vue 3.5.40、JavaScript module、Node 22.23.3 LTS系 | Vue公式のビルドなし利用も可能な構成を基に、runtimeのrender関数を使う。Vite・SFC・TypeScript・Router・storeはこの1画面に不要。npmで固定したVue runtimeを同じホストから配信し、CDNへ実行時依存しない。3.5.40はローカルcacheにある固定版。最新3.5.44の存在も確認したが、この版の検証と更新は別扱い。[Vue Quick Start](https://vuejs.org/guide/quick-start.html)、[公式3.5.40](https://github.com/vuejs/core/releases/tag/v3.5.40)、[Nodeのサポート状況](https://nodejs.org/en/about/previous-releases) |

Goの依存版は `go.mod`、Frontendは `package.json` と `package-lock.json` を参照する。sqlcはバージョンを指定した `go generate` で実行する。未取得の依存のchecksumを手で作らない。依存取得・生成・実行の状況は[検証記録](verification-issue-53.md)で区別する。

配置は `backend/cmd/api`、`backend/cmd/user-create`、共有する `backend/internal/auth`、HTTP入口 `internal/httpapi`、DB adapter `internal/store`、sqlc生成先 `internal/db`、SQLを `backend/db`、UIを `frontend` とする。将来のWorker等は先に作らない。ローカルのNodeサーバーは静的ファイル配信と `/api/` のlocalhost proxyのみを行う。本番のnginx・systemd・HTTPS構築は #21。

## パスワードと管理コマンド

`golang.org/x/crypto/argon2` のArgon2idをAPIと管理コマンドで共用する。bcryptは72byte上限があり、単純SHA-256はパスワード推測に速すぎるため見送る。`m=19456 KiB, t=2, p=1`、saltは `crypto/rand` の16byte、出力32byte、version=19とし、PHC形式でパラメーター・salt・hashを保存する。[Go argon2](https://pkg.go.dev/golang.org/x/crypto/argon2)、[OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)を確認した。OWASPの最低構成を用い、公開前には対象ホストで時間・メモリを測る。

入力は15〜128 Unicodeコードポイント・最大512 UTF-8 byte。空白・文字種を制限せず、正規化・trim・暗黙切り捨てをしない。MFAなしの長さについて[OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)を参照する。ログインIDは既存仕様どおりASCII英大文字だけ小文字化し、`[a-z0-9_-]{1,64}` を検証する。ハッシュ検証では既知の固定パラメーターとsalt/hash長を先に確認し、DB破損時に任意のメモリ量を要求させない。

管理コマンドは `--login-id` のみを受け取り、`/dev/tty` と [x/term ReadPassword](https://pkg.go.dev/golang.org/x/term#ReadPassword) でパスワードと確認入力を非表示で読む。パスワード用引数、環境変数、標準入力の自動fallbackは設けない。バイト列は使用後clearするが、Goのstringやハッシュ処理内のコピーを完全に消去する保証はない。重複IDはDB一意制約のSQLSTATEと制約名から判別し、Userを作らず明示する。出力は成否だけで、ハッシュ・接続文字列・入力値を出さない。

APIのArgon2同時実行は2件までとしメモリ使用を抑える。これはアカウント/IP別の試行回数制限ではなく、待機は要求timeoutで打ち切る。未知IDにもdummy hash照合を行う。試行制限・認証専用イベントは #49に残す。

## CookieとCSRF

トークンは `crypto/rand` 32byteを64文字の小文字hexへ符号化する。DBには元byte列のSHA-256だけを保存する。ULIDは記録の識別専用で、認証情報として使用しない。高エントロピートークンにはパスワード用の遅いhashを使う必要がなく、索引検索できるSHA-256を使う。セッション保存成功後だけCookieを発行し、ログイン時は毎回新しいトークンを生成する。既存Cookieを昇格・再利用しない。旧セッションのログアウト処理は #54。

| 項目 | 実装 |
|---|---|
| HTTPS | `COOKIE_MODE=https`、`APP_ORIGIN=https://実ホスト`。`__Host-watchbrief_session`、Secure、HttpOnly、Path=/、Domainなし、SameSite=Lax |
| ローカルHTTP | 明示的な `COOKIE_MODE=local-http`、Originのhostはlocalhostまたは127.0.0.1のみ。Cookie名は `watchbrief_session_local`、Secureのみfalse。他属性は同じ。架空データ専用 |
| 保存・更新 | `Max-Age=34560000`（400日）。ログインと認証済み `/api/me` 成功応答で同じtokenの期限を更新。DB側に絶対期限・無操作期限を持たせない |
| 再起動 | 永続Cookieによりブラウザ再起動を考慮。APIプロセスに認証状態を保持せず、再起動後も同じDBのhashで照合する |
| 制限 | ブラウザの保存制限・利用者の削除・長期間アクセスなしのCookie自然消失は防げず、無期限保持は保証しない。Cookieが消えてもDBへ削除通知は来ないため、孤立セッション整理は未実装 |

Cookieの[MDN Set-Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie)と[Chromiumの400日制限](https://developer.chrome.com/blog/cookie-max-age-expires)を参照する。Cookie更新はブラウザの保存更新であり、サーバー側の時間失効導入ではない。HTTPSの判定をリクエストの `X-Forwarded-Proto` に任せず、起動設定で固定する。TLS終端・本番nginx・ホスト名の接続確認は #21へ残す。

CSRFは[OWASPのAJAX向けcustom request header方式](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html#employing-custom-request-headers-for-ajaxapi)を採用する。セッション用CSRF token・匿名session・double submit cookieを増やさず、次をログイン前から共通middlewareで適用する。

- GET/HEAD以外では `Origin` が設定した完全なOriginと一致することを必須とする。欠落・null・別ポート・別ホスト・同一siteの別originを拒否する。Refererから推測しない。
- `X-WatchBrief-CSRF: 1` と `Content-Type: application/json` を要求する。通常の外部HTML formでは専用headerを付けられない。
- CORS許可headerを一切返さない。外部JavaScriptによる専用header送信のpreflightを許可しない。
- `Sec-Fetch-Site` が存在する場合は `same-origin` のみ許可する。非対応clientではOrigin＋専用headerの必須検証を維持する。

このheader値自体はSecretではなく、同一originのJavaScriptだけが送れることとCORS非許可で防ぐ。ブラウザ以外のclientはOriginを偽装できるが、別途有効なパスワード／Cookieが必要であり、CSRF防御はそれらの代わりではない。XSSが成立すれば同一originで操作できるため、UIはrender関数でテキストを扱い、HTML挿入を使わず、外部scriptを許可しないCSPを設定する。今後の変更APIも共通middlewareから外さず、GETに業務変更を追加しない。POSTログインと本人情報GETのみを今回公開する。

## DB権限・Secret・障害

DBの型・制約・権限表は[DB設計の物理スキーマ](database-design.md#issue-53の認証物理スキーマ)を正とする。bootstrap以外にsuperuser・CREATEDB・CREATEROLEを与えない。APIはDDL所有権・migration資格情報・User作成資格情報を持たない。

APIと管理コマンドは、各々のDB role名とパスワードを含む別々の0600 JSONを、`DB_CONFIG_FILE` の**パス**から読む。ファイルはGit外、親0700、`scripts/write-db-config.py` のgetpassで作成する。JSONを標準出力・ログへ出さず、DSNへ連結しない。今回は接続host=127.0.0.1・sslmode=disableをローカル同居DB限定で使い、ネットワーク越しのDB用TLS設定は将来のRDS分離時に扱う。ローカル同一OSユーザー内でのファイル保護はプロセス間の完全隔離ではなく、本番のOSユーザー分離・systemd credentialsは #21で具体化する。

| 状況 | 応答と保存・ログ |
|---|---|
| 不正入力・JSON/サイズ超過 | 400、固定code。認証・保存へ進めない |
| CSRF不正／JSON以外 | 403／415。DBアクセス前に拒否 |
| 未知ID／誤パスワード | 共通401 `invalid_credentials`。Cookie・セッションを作らない |
| Cookie不在・形式不正・重複・DB該当なし | 401 `unauthenticated`。本人情報を返さない |
| DB・乱数・hash破損・想定外panic | 500 `internal_error`。Cookieを発行・更新しない。固定のoperation/stage/result、request ID、versionと時刻のJSONログ |
| DB接続不能・設定不正で起動 | 固定分類のJSONログと非zero終了。Secretやdriver error全文を出さない |

APIは4096byteのbody上限、DB接続5秒、要求処理10秒、HTTP read/write15秒を設定する。認証状態の時間失効とは別のI/O上限である。DB保存とCookie送信を原子的にはできないため、保存直後の通信断ではCookieを得られない孤立sessionが残り得る。ログインを自動retryせず、利用者が再試行する。本人情報には明示したID・login_id以外のDB値を渡さず、レスポンスは `Cache-Control: no-store`。パスワード・Cookie・hash・headers・body・URL・driverエラーの生値をログへ渡さず、net/httpの内部ログも固定分類へ置き換える。
