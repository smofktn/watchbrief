# ローカルでログインを確認する

> 旧案のGoコード（auth/config/httpapi/store）は、内容を変えず[参考ディレクトリ](../reference/issue-53-previous-proposal/README.md)へ隔離した。以下のコード配置・実行例は当時の記録であり、現在の起動手順として実行しない。

> **Issue #53の既存案について**：以前のAIが作成した未完成の実装案と、それに伴う文書変更を参考として保持している。具体バージョン、ハッシュ、Cookie・CSRF、migration等は、今回利用者と確認したもの以外は採用済みと扱わない。過去の検証結果も今回の実動作確認とは区別する。現在の進め方は[作業計画](work-plan.md#利用者主体で子issueを進める運用合意済み)を参照する。

Issue #53の架空User作成→ログイン→本人情報表示を対象とする。Report生成・設定・ログアウトはまだ使えない。これは実行手順であり、このセッションで全手順を実行できたという意味ではない。[確認状況](verification-issue-53.md)を参照する。

## 前提と依存

Go 1.27.3、PostgreSQL 18.6（`initdb`、`pg_ctl`、`createdb`、`psql`）、Node 22.23.3とnpmを用意する。OS全体への導入・Dockerへのアクセスが許可されていると仮定しない。必要ならユーザー領域の公式バイナリを使う。Goのchecksumは公式配布情報で照合する。依存取得とlocalhostへのlistenが許可された環境で実施する。すべてリポジトリルートから実行し、`umask 077` を使う。コマンドにSecretの値を付けない。

```bash
umask 077
mkdir -p .local/db-socket .local/secrets bin /tmp/watchbrief-supervision/issue-53
chmod 700 .local/db-socket .local/secrets
(cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate)
(cd backend && go mod tidy)
(cd backend && gofmt -w cmd internal)
(cd backend && go test -race ./... && go vet ./...)
(cd backend && go build -o ../bin/api ./cmd/api)
(cd backend && go build -o ../bin/user-create ./cmd/user-create)
npm --prefix frontend ci
npm --prefix frontend run build
npm --prefix frontend test
```

以前のAI担当セッションではGoがなく、`go.sum` とsqlc生成物を生成できなかった。以下はその案を実行する場合の参考であり、今回の実装をこの手順で完成させる決定ではない。上のsqlc実行 → `go mod tidy` により生成し、diffをレビューしてからコミット候補に含める。checksumや生成結果を手書きで捏造しない。通常の変更時はlockfileに基づく `go mod download` と `npm ci` を使い、依存更新の意図がないのにtidy/installで版を変更しない。sqlc生成の再現確認は `go generate ./internal/store` 後の `git diff --exit-code -- internal/db` で行う（初回生成物をレビュー・追跡した後）。

## 架空データ専用DBの初期化

新しい `.local/pgdata` のみを対象にする。既存データディレクトリを削除・初期化し直さない。bootstrap用DBパスワードは `initdb` の非表示promptで入力し、メモやGit・制御ログへ転記しない。bootstrap資格情報はAPIへ渡さない。

```bash
initdb -D .local/pgdata -U watchbrief_bootstrap --auth=scram-sha-256 --pwprompt --encoding=UTF8 --locale=C
```

`.local/pgdata/postgresql.conf` の末尾に以下の**秘密値を含まない**ローカル専用設定を加える。socket directoryの絶対パスは自分の作業フォルダーの `.local/db-socket` を指定する。listenは127.0.0.1のみ、socket親は0700とし、TCP・socketともSCRAM認証を維持する。

```conf
listen_addresses = '127.0.0.1'
port = 55432
# unix_socket_directories = '/absolute/path/to/project/.local/db-socket'
password_encryption = 'scram-sha-256'
log_statement = 'none'
log_min_error_statement = 'panic'
log_error_verbosity = 'terse'
log_parameter_max_length = 0
log_parameter_max_length_on_error = 0
log_connections = off
log_disconnections = off
timezone = 'UTC'
```

`unix_socket_directories` はコメントを外して実パスを設定する。接続では以下のとおりTCPを明示する。起動ログも指定ディレクトリ外へ出さない。

```bash
pg_ctl -D .local/pgdata -l /tmp/watchbrief-supervision/issue-53/postgres.log start
createdb -h 127.0.0.1 -p 55432 -U watchbrief_bootstrap -W watchbrief
psql -X -h 127.0.0.1 -p 55432 -U watchbrief_bootstrap -W -d watchbrief -f backend/db/bootstrap.sql
psql -X -h 127.0.0.1 -p 55432 -U watchbrief_migrator -W -d watchbrief -f backend/db/migrate.sql
```

`-W` はDBパスワードを非表示promptで読む。bootstrap SQLは初回のみ、専用DBにロール・権限を作り、最後に3ロールそれぞれのパスワードをpsqlの `\password` で非表示入力する。パスワードをSQL文・コマンド引数・shell履歴へ入れない。途中失敗時は重複ロールを無条件に削除せず、作成済みの状態を調べて未実行部分だけを管理者が再実行する。

migrationはAPI起動とは分離し、`watchbrief_migrator` がtransaction内で `SET ROLE watchbrief_owner` を実行する。版1を記録済みなら再作成しない。SQL失敗時は版とDDLを一緒にrollbackする。`000001_auth.down.sql` は**認証データ全削除**となるため通常起動・検証では実行しない。空の検証DBでrollback試験をする場合だけ、所有者権限のtransactionでdownと `DELETE FROM schema_migrations WHERE version=1` を同時実行する。実データの切り戻し手段と考えない。

## 接続設定とUser作成

API用とUser作成用に、先ほど設定したそれぞれのDB roleのパスワードを非表示promptへ入力する。アプリで使うログインパスワードとは別物である。

```bash
python3 scripts/write-db-config.py --role watchbrief_api --file .local/secrets/api.json
python3 scripts/write-db-config.py --role watchbrief_user_admin --file .local/secrets/user-admin.json
DB_CONFIG_FILE="$PWD/.local/secrets/user-admin.json" ./bin/user-create --login-id fictional_user
```

最後のコマンドでは架空Userのログインパスワードを非表示で2回入力する（15〜128 Unicode文字、最大512 UTF-8 byte）。入力は引数・標準出力・ログに出さない。接続JSONは親0700・ファイル0600、既存ファイルへの上書きは拒否する。内容を `cat` したりIssueへ貼り付けない。APIへは `api.json` だけを渡す。DB role・入力条件・採用理由は[認証の実装方式](auth-implementation.md)を参照する。

## 起動と画面確認

端末1でAPIを起動する。

```bash
DB_CONFIG_FILE="$PWD/.local/secrets/api.json" APP_ORIGIN=http://127.0.0.1:5173 COOKIE_MODE=local-http ./bin/api
```

端末2でFrontendを起動する。

```bash
npm --prefix frontend run dev
```

`http://127.0.0.1:5173` を開き、作成した架空Userでログインする。画面に小文字化済みログインIDが出れば本人情報APIまで通っている。パスワード・Cookie・レスポンスheaderをスクリーンショットやログへ残さない。127.0.0.1とlocalhostは別Originなので混在させない。

APIの `GET /api/me` は未認証401。`POST /api/login` はOrigin・専用header・JSONが必要。Vueはfetchのsame-origin Cookie送信を使い、トークンをJavaScriptの保存領域やレスポンスJSONへ露出しない。

ブラウザを完全に終了して再起動し、同じプロファイルで本人情報が出ることを確認する。Cookie消去機能やプライベートモードを使う場合は維持されない。APIをCtrl-Cで止め、同じコマンドで起動してからページを再取得し、同じCookieでログイン状態が維持されることも別に確認する。

## 再現できる検証

以下は独立した検証手順であり、成功結果は[検証記録](verification-issue-53.md)と照合する。実行前に上で起動したAPIを停止し、8080を空ける。

```bash
python3 scripts/verify-login.py --api-config .local/secrets/api.json --admin-config .local/secrets/user-admin.json
psql -X -h 127.0.0.1 -p 55432 -U watchbrief_bootstrap -W -d watchbrief -f backend/db/verify.sql
```

Python検証はランダムな架空ID・パスワードをメモリ内で作り、実User作成コマンドの非表示TTY入力、重複ID拒否、未認証、誤パスワード、入力不正、CSRF拒否、Cookie属性、本人情報、**実APIプロセス停止・再起動後の同じCookieによる認証**、ログ・レスポンス非混入を確認する。パスワードを標準出力へ出さず、Cookie jarはメモリのみ。作った架空User・sessionは専用DBに残る。スクリプトはUser削除・ログアウトを追加しない。

SQL検証はtransaction内の架空データでULID/login条件、重複ID・token、hash長、FK・RESTRICTと、APIからのCREATE/ALTER/CREATE ROLE/所有者昇格/CREATE SCHEMA/TEMP TABLE/User作成・変更の拒否を確認してrollbackする。SQL側は `SET ROLE`によるDB内の権限試験とownerへのmembership確認であり、別roleの接続認証試験とは区別する。Python側では保護JSONから接続して実API roleのDDL・CREATE ROLE・SET ROLE owner等を試し、42501拒否を確認する。psql子プロセスへは一時的なPGPASSWORD環境変数で渡し、引数・ログ・shell履歴へ値を出さない。ログにSQLやhash値を出すpsqlの `-a`・`-e`、PostgreSQLのstatement loggingを有効にしない。

本番HTTPS cookie・nginx・TLS・複数端末・ブラウザの長期保存上限はこれらだけで検証済みにならない。将来HTTPSでは `COOKIE_MODE=https` と実Originを設定する。Secureをforwarded headerから動的に切り替えない。今回外部公開は行わない。

終了時はAPI・FrontendをCtrl-Cで止め、DBを止める。データ・設定は保持する。

```bash
pg_ctl -D .local/pgdata stop -m fast
```
