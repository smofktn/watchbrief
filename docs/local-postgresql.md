# ローカルPostgreSQL

ローカル開発ではPostgreSQLだけをDockerで動かし、Vue・Go・管理コマンド・gooseはホスト上で実行する。本番EC2の配置方針は変更しない。起動設定はルートの `compose.yaml`。操作の入口は `backend/Makefile` にまとめ、ルートのMakefileは同名ターゲットへ委譲する。

## 起動

Backendディレクトリで実行する。ルートの同名ターゲットからも呼べる。Docker Engine・ComposeとPython 3が必要。

```bash
make db-prepare
make db-up
make db-status
```

準備スクリプトは `.local/postgres-admin-password` に初期管理用のランダムパスワードを作る。ファイルはmode 600とし、値を表示せず、既存ファイルを上書きしない。`.local/` はGit管理対象外。パスワードをコマンド引数・履歴・ログ・Issueへ転記しない。

公式イメージ `postgres:18.6-bookworm` を固定して使う。PostgreSQL 18以上の公式イメージに合わせ、`/var/lib/postgresql` へ専用の名前付きvolumeをマウントする。ホスト側の公開先は `127.0.0.1:5432` に限定する。既存のDBが同じポートを使っている場合は、既存サービスを停止せず、設定の調整を相談する。

## データと認証

| 項目 | 用途 |
|---|---|
| DB名 `watchbrief` | アプリ用のローカルDB |
| 管理ロール `postgres` | 初期DB管理専用。API・User作成の接続には使わない |
| `.local/postgres-admin-password` | 初期管理ロールの秘密。Compose secretから `POSTGRES_PASSWORD_FILE` で読み込む |
| `watchbrief-local_postgres-data` | PostgreSQLの永続データ。Gitには含めない |

TCP接続は初期化時の `--auth-host=scram-sha-256` と `POSTGRES_HOST_AUTH_METHOD=scram-sha-256` でパスワード照合を要求する。コンテナ内の管理用Unix socket接続は公式イメージの初期動作に従いtrustとなるため、TCPの認証試験は `-h 127.0.0.1` を必ず指定する。healthcheckのpg_isreadyは待受の確認であり、パスワード認証の成功を証明しない。

これらの初期化変数は空のvolumeの初回だけに適用される。ファイルを変えて再起動しても、既存DBのパスワード・認証ルールは更新されない。パスワードファイルを失くした場合は、volumeを消したり秘密を作り直したりせず、既存DBと認証状態を確認して復旧する。

## 用途別の初期設定

DB起動後、リポジトリルートで一度だけ実行する。これはpostgresによる初期設定であり、gooseのテーブルmigrationとは別の管理操作。

```bash
make db-bootstrap
```

`backend/db/bootstrap-local-roles.sql` のロール・schema作成と、psqlの非表示パスワード入力を1つのトランザクションで実行する。パスワードは用途別の乱数ファイルを600で作り、標準入力からpsqlへ渡す。SQLの引数に平文パスワードを埋め込まず、psqlがクライアント側で暗号化して設定する。出力には秘密値やDBの詳細エラーを表示しない。

| 接続ロール | ローカル秘密ファイル（`.local/` 内） | 現在の権限 |
|---|---|---|
| `watchbrief_api` | `watchbrief-api-password` | DB接続・app schemaのUSAGE。テーブル権限は下記参照。DDL・TEMP・他ロールへの切替は不可 |
| `watchbrief_user_creator` | `watchbrief-user-creator-password` | DB接続・app schemaのUSAGE。テーブル権限は下記参照。DDL・TEMP・他ロールへの切替は不可 |
| `watchbrief_password_resetter` | `watchbrief-password-resetter-password` | 対象ID検索・hash更新・全session削除だけ。詳細は[再設定](password-reset.md#最小db権限と索引) |
| `watchbrief_migrator` | `watchbrief-migrator-password` | DB接続・app schema所有者としてその中のDDL |

4ロールともsuperuser・DB作成・他DBロール管理・replication・RLSバイパスを許可せず、相互のmembershipも与えない。DB自体の所有者はpostgresのままとし、PUBLICのwatchbriefへの接続・TEMP、public schemaへの権限は外す。schema所有者用の別NOLOGINロールは今回追加せず、migration用が直接app schemaを所有する。これらの権限制御は専用DB `watchbrief` の範囲である。

接続時の `search_path` は `app, pg_catalog`。API・User作成用はappにオブジェクトを作れず、migration用だけがschemaを管理する。テーブルとテーブル単位の権限は、次のgoose migrationで管理する。

すでに同名ロール・schemaがある場合は失敗してロールバックし、再設定・パスワード更新は行わない。失敗時はファイルやロールを削除してやり直さず、現在の状態を確認する。通常の再起動時にこのスクリプトを実行する必要はない。

旧AI案の `backend/db/bootstrap.sql`・migrationは参考として保持するが実行しない。現在の手順とロール構成はこの文書と `bootstrap-local-roles.sql` を参照する。認証テーブルは現在のgoose migrationで作る。User作成コマンドとログインAPIは、それぞれの専用ロールで接続する。本人情報APIもAPI専用ロールでUser・セッションをSELECTし、DBへ書き込まない。

## テーブルmigration

Backendディレクトリで実行する。ルートの同名ターゲットからも呼べる。Go APIの起動時には実行せず、運用者がmigration専用ロールで明示的に適用する。

```bash
make goose-install
make db-migrate
make db-migration-status
```

CLIはgoose v3.29.0。インストーラーはLinux amd64の公式バイナリを公式リリースのSHA-256と照合し、Git対象外の `.local/bin/goose` へ置く。既存バイナリが異なれば上書きしない。Go・Nodeは引き続きasdfで管理する。

現在のmigrationは `backend/db/goose/` の番号付きSQL（認証・設定・Report・再設定権限と索引）。旧案の `backend/db/migrations/` は読み込まない。Up/Downを同じSQLに置き、gooseの通常のトランザクションで適用する。独自のDB版管理は作らず、適用状態は `app.goose_db_version` へ保存する。

接続パスワードは既存のmigration用秘密ファイルから読み、600の一時passfileでCLIへ渡す。パスワードを引数へ入れず、ローカル固定の接続先だけを使う。継承したPG/GOOSE設定と自動.env読込を無効化する。完了・失敗時は一時ファイルを除去し、DB接続エラーの詳細は出力しない。このラッパー経由の同時実行はローカルfile lockで拒否する。別ホストやCLI直接実行の並行制御を保証するものではない。

| 対象 | API用 | User作成用 |
|---|---|---|
| `app.users` | SELECT | INSERT |
| `app.auth_sessions` | SELECT・INSERT・DELETE | 権限なし |
| `app.goose_db_version` | 権限なし | 権限なし |

上表のAPI・User作成用にはUserの更新・削除とセッションの更新を付与しない。再設定専用ロールの権限は後述する。ログアウトに必要なセッションDELETEだけを版2で追加する。テーブル所有者はmigration用であり、API/User作成用は所有者へ切り替えられない。

```bash
make db-verify
```

専用ローカルDBで、架空データを各トランザクション内へ入れて制約・許可/拒否を検証し、ロールバックする。実際のパスワードハッシュ・生トークンは使わず、レスポンスや行データも出力しない。これはアプリの認証試験の代わりではない。

通常ラッパーは `up`・`status`・`version` だけを公開する。版1のDownは認証データを削除し、版2のDownはDELETE権限を取り消す。通常操作や確認手順にDownは含めない。失敗時にreset・volume削除・版管理行の手編集を行わず、失敗した版とDB状態を確認する。Downは削除したデータを復元しない。

## ログアウト用の権限追加（Issue #54）

`make db-migrate` で版2を適用する。変更は `watchbrief_api` へのauth_sessions DELETE権限追加だけで、既存行は削除・更新しない。API起動へmigrationを混ぜない。`make db-verify` は更新後の権限も検証し、試験データはロールバックする。[保存処理と権限の詳細](database-design.md#issue-54のログアウト保存処理)を参照する。

## 停止と再開

```bash
make db-stop
make db-up
```

停止してもデータはvolumeに残る。起動・復旧手順にvolume削除は含めない。メジャー版の変更ではデータ移行が必要なので、イメージの数字だけを変更しない。

公式情報：[PostgreSQLの版管理](https://www.postgresql.org/support/versioning/)、[Postgres公式イメージ](https://hub.docker.com/_/postgres)。実行した認証・永続化の検証結果と未確認の範囲はIssue #53へ記録する。

## 再設定用の専用ロール追加（Issue #58）

新規環境の `make db-bootstrap` は `watchbrief_password_resetter` も作る。以前の3ロールで構築済みの環境では、postgresの初期管理操作として次を**1回だけ**実行する。API・既存User・業務データは変更せず、新しい再設定専用ロールと接続設定だけを追加する。

```bash
make db-provision-password-resetter
make db-migrate
```

追加操作は `.local/watchbrief-password-resetter-password` を600で新規生成し、既存ファイルは維持する。既存ロールがあればtransaction全体を失敗させ、パスワードを上書きしない。新規bootstrap済み・追加済みの環境では再実行せずmigrationだけを実行する。失敗時にrole・ファイル・volumeを削除せず、現在の状態を確認する。

provisioningはCREATE ROLE・CONNECT・app USAGE・search_pathと秘密パスワード設定を同transactionで行う。既存のAPI・作成者・migrationの秘密ファイルは変更しない。goose用ロールへCREATEROLEを追加しない。版7が専用ロールへのカラム/テーブル権限、版8がsession user_id索引を追加する。通常の起動へどちらも混ぜない。権限と競合の設計は[再設定](password-reset.md)を参照する。
