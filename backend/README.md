# Backend

現在の `cmd/api` は、PostgreSQLのUserを共通Argon2idで照合し、セッション保存後に204と認証Cookieを返す。`GET /api/me` はCookieのトークンから本人のログインIDだけを返し、同じCookieの期限を更新する。

asdfでルートの `.tool-versions` のGoを導入した後、このディレクトリで実行する。

```bash
make dev
```

`backend/Makefile` がAPI・管理コマンド・DB操作を担当する。`make help` で一覧を表示する。ルートからは `make backend-dev` 等で同じMakefileを呼べる。

APIは `127.0.0.1:8080` で待ち受ける。既定のAPP_ORIGINは `http://127.0.0.1:5173`。変更する場合は `make dev APP_ORIGIN=画面のOrigin` とする。APP_ORIGINには画面のOrigin（プロトコル・ホスト・ポート、末尾スラッシュなし）を指定する。設定なし・不正な設定では起動しない。HTTPのOriginはlocalhost・ループバックIPに限定する。終了はCtrl+C。

先に[ローカルDB](../docs/local-postgresql.md)の初期設定・migrationと架空User作成を済ませる。APIは専用ロールwatchbrief_apiで接続する。DBパスワードの既定ファイルは `../.local/watchbrief-api-password`、変更は `API_PASSWORD_FILE` にパスだけを指定する。`API_ADDR` で待受を変更できるが、ループバックIP・有効な数値ポートに限定する。依存取得は `make deps`（ルートでは `make backend-deps`）。

別の端末から、秘密情報を使わず入力不正を確認できる。

```bash
make check-input
```

期待する応答は `400` と `error.code: invalid_request`。Originを省略すると、JSON解析より前に `403 csrf_rejected` となる。実際のパスワードをcurlの引数・履歴へ書かない。

## 責務と検証範囲

- `internal/domain`：ASCIIのID正規化・条件、パスワード15〜128コードポイントの検証。HTTP・DBには依存しない。パスワードのtrimや大小文字変換は行わない。NFC正規化を文字数確認前に適用する。制御文字・弱いパスワードリスト等の追加拒否はまだ未実装。
- `internal/transport/httpapi`：Origin・Go標準CSRF防御、JSON解析、ドメイン検証、固定のJSONエラー。信頼するOriginはAPP_ORIGINの1つだけとし、それとは別にOriginヘッダー1件・完全一致を必須にする。
- `internal/usecase`：User取得・共通ハッシュ照合・トークン生成・セッション保存。HTTPとSQLの詳細には依存しない。
- `internal/adapter/postgres`：sqlcによるUser検索・セッションINSERT。APIの接続プールは最大4接続。
- `cmd/api`：DB・共有ハッシュ部品・ユースケース・HTTPの組み立てと起動・終了。migrationは自動実行しない。

```bash
make test
```

`make test` はBackend全体（`./...`）をrace付きで検証する。実DBは `make test-db`、端末からの作成は `make verify-user-create` で個別に確認する。旧AI案のGoコード（auth/httpapi/store/config）は内容を変えず[参考ディレクトリ](../reference/issue-53-previous-proposal/README.md)へ隔離し、通常のパッケージ探索に含めない。旧DBファイルは元の場所に参考として保持し、現在のmigration/sqlcでは読み込まない。

DBの起動・用途別ロール・gooseによるテーブル作成は[ローカルPostgreSQL](../docs/local-postgresql.md)で管理する。ログイン受付からDBへ接続済み。本人情報取得も同じAPI用ロールで接続する。

共通Argon2id部品は `internal/security/password`。API内では共有インスタンスを使い、別プロセスのUser作成でも同じ実装を使う。初期プロファイル・テスト・キャンセル等は[共通ハッシュ](../docs/password-hashing.md)へまとめる。管理コマンドはこの部品を使用する。未知IDでも起動時に生成したダミーハッシュを照合し、誤パスワードと同じ401を返す。通信全体の処理時間が常に同一になる保証ではない。

## 今回の仮実装と保留事項

入力検証を小さく確認するため、HTTP受付は暫定的に以下を使う。最終API契約への一括採用を意味しない。[API仕様](../docs/API仕様/post-api-login.md)と照合し、残る詳細を今後確定する。

- 本文上限4KiB。application/jsonを受け付ける。サイズ超過・対応外のContent-Typeは暫定的に400 invalid_request。提案中の413/415はまだ採用しない。
- JSONは1つのオブジェクトだけ。必須2項目の正確な名前を要求し、未知項目・重複項目・null・末尾の別JSONを拒否。NFC正規化は共通ドメイン処理で実行する。不正UTF-8・対にならないUTF-16サロゲートも拒否する。
- JSON応答にContent-TypeとCache-Control: no-storeを付ける。入力不正・CSRF拒否の未合意文言は検証用の固定文言。
- 入力検証後に実DBのUserと照合し、誤パスワード・未知IDは401、内部障害は500。成功時だけセッションを保存しCookieを発行する。

FrontendからVite中継でログインできる。Cookie名・属性・文字列表現は[API共通仕様](../docs/API仕様/README.md)を参照する。画面はログイン後・初回表示・再読み込み時に本人情報を取得する。

## sqlcとUser保存の準備

Backendディレクトリで実行する。ルートでは `make sqlc-install`・`make sqlc-generate` でも同じ処理を呼べる。

```bash
make sqlc-install
make sqlc-generate
```

sqlc v1.31.1の公式Linux amd64配布物を固定し、公式リリースのSHA-256と照合する。設定は `backend/sqlc.yaml`。`db/sqlc-schema.sql` のschema宣言と、現在の `db/goose` を読み、`db/current-queries` のSQLから `internal/adapter/postgres/generated` へ生成する。テーブルDDLを別ファイルに複製せず、gooseのDownは生成時に無視する。生成はDBを更新する操作ではない。旧案の `db/queries` と `db/migrations` は読み込まない。

DBドライバーはpgx v5.12.0。`internal/adapter/postgres` がドメインのUserをsqlcの型へ変換し、ドライバーエラーから重複ログインIDとその他の保存エラーを分類する。SQL・DBの詳細やハッシュを内側のエラーへ含めない。User作成はINSERTだけで完結させ、SELECT権限が必要となるRETURNINGを使わない。

ローカルの接続先・DB・User作成用ロールを固定し、既存の600の秘密ファイルからDBパスワードをメモリへ読み込む。接続資格情報をコマンド引数やログへ含めない。HTTP・パスワード照合・アプリのログインUserの認可はこのDBアダプターで扱わない。

DB準備後、Backendディレクトリで実行する。

```bash
make test-db
```

User作成用の実TCP接続で、架空データのINSERTと重複IDを確認しロールバックする。API用接続でINSERT拒否と安全なエラー変換も検証する。これらは保存アダプターの試験であり、下記の端末入力からの作成試験とは区別する。

## 架空Userの作成

[ローカルDBの初期設定・migration](../docs/local-postgresql.md)を完了した後、Backendディレクトリから実行する。

```bash
make user-create LOGIN_ID=fictional_user
```

架空のログインIDを引数へ指定する。英大文字は小文字へそろえる。パスワードを2回、端末で非表示入力する。入力欄に表示されなくても受け付けているので、入力後にEnterを押す。パスワードを引数へ付けたり、パイプやスクリプトへ書いたりしない。

作成時・確認入力ともNFCでそろえた後に15〜128コードポイントを検証する。前後空白・大文字小文字・全角半角を勝手に同一視しない。正規化後の入力が一致すれば共通Argon2idでハッシュ化し、別のULIDを生成して、User作成専用ロールで `app.users` へ1件INSERTする。DB接続は入力検証・ハッシュ・ID生成の後まで開始しない。成功時は作成完了を表示する。

不一致・入力不正・端末なし・Ctrl+Cでは保存しない。重複IDでは既存Userやそのパスワードを変えない。DB通信中の失敗では作成結果が不明な場合があるため、結果不明の表示が出たら、既存の状態を確認してから再実行する。自動再試行・上書きは行わない。

接続先は専用ローカルDB、ロールはwatchbrief_user_creatorに固定する。DBパスワードの既定ファイルは `../.local/watchbrief-user-creator-password`。必要なら `make user-create LOGIN_ID=fictional_user DB_PASSWORD_FILE=ファイルパス` としてパスだけを指定できる。これはアプリのログインパスワードとは別の秘密。

端末入力は現在Linux用。x/termとUnixの端末制御を使い、入力待機をcontextで中断できるようにする。終了時には元の表示設定を戻し、非表示で読めない環境で表示入力へ切り替えない。CLIは生パスワードをファイル・ログへ保存しない。操作の15秒timeoutは端末入力後のハッシュ・DB保存に対するもので、利用者の入力待ち時間やログインセッションに失効時間を設けるものではない。

Backendまたはリポジトリルートで実行する。Makefileが検証用CLIをビルドしてから、疑似端末の試験を呼び出す。

```bash
make verify-user-create
```

この試験にはLinuxのPTYとlibargon2が必要。検証用の架空User1件を保持し、再実行時は同じUserを再利用する。資格情報は `.local/user-create-verification.json`（600・Git対象外）だけに保存し値は表示しない。通常のUser作成コマンドが平文パスワードを保存するという意味ではない。失敗ケースではDBにUserが増えず、重複ケースでは既存値が変わらないこと、非表示入力・Ctrl+Cの表示復元と、保存ハッシュの独立実装での照合を確認する。

## ログインAPIの実DB検証

```bash
make verify-login-api
```

`make verify-user-create` の非公開検証用Userを再利用し、別ポートに検証APIを起動する。未知ID・誤パスワード・CSRF拒否、NFC照合、Cookie属性、トークンとDBハッシュの対応、再起動後のDBセッション保持を確認する。1回に検証セッション2件を残す。値は表示せず、既存Userやセッションを削除しない。本人情報APIでの認証・同じCookieの期限更新・API再起動後の再認証も確認し、本人情報取得でDBのセッション行が変わらないことを検証する。
