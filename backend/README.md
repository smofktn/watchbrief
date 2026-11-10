# Backend

初めての構築から画面・障害復旧までの入口は[ローカル起動・確認手順](../docs/verification-issue-17.md)。`make verify-issue17` は通常DBを止めず、専用の使い捨てDBで横断検証します。

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
- `internal/adapter/postgres`：sqlcによるUser検索・セッションINSERT／検索／対象削除。APIの接続プールは最大4接続。
- `cmd/api`：DB・共有ハッシュ部品・ユースケース・HTTPの組み立てと起動・終了。migrationは自動実行しない。

```bash
make test
```

`make test` はBackend全体（`./...`）をrace付きで検証する。実DBは `make test-db`、端末からの作成は `make verify-user-create` で個別に確認する。旧AI案のGoコード（auth/httpapi/store/config）は内容を変えず[参考ディレクトリ](../reference/issue-53-previous-proposal/README.md)へ隔離し、通常のパッケージ探索に含めない。旧DBファイルは元の場所に参考として保持し、現在のmigration/sqlcでは読み込まない。

カバレッジは `make coverage`（ルートでは `make backend-coverage`）で計測する。現在のGo module全体を `go test -coverprofile` のpackage標準の範囲で計測し、sqlc生成コードやCLIも分母に含める。生profileは `.local/coverage/backend/coverage.out`、行別表示は同じ場所の `index.html`。`go tool cover -func` で関数別と全体のstatement coverageを表示する。ルートの `make coverage` はFrontendも計測する。生成物はGit対象外。

この数値は非DBのGo試験だけを表す。アダプターの偽I/O試験では、値の変換、所有者・hashの引き渡し、失敗分類、処理順序とtransaction終了を検証する。SQLの所有者条件・DB制約・競合ロックの実際の動作や端末の非表示入力は、実DB・PTY・隔離環境の試験で別に確認する。coverageの割合だけをこれらの動作保証には使わない。

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

## ログアウトAPI（Issue #54）

`POST /api/logout` はCookieで指定されたセッションだけを物理削除してからCookieを削除し、204を返す。Origin必須・完全一致とGo標準CSRF防御をログインと共通化している。本文は不要。Cookie欠落・不正・重複は401、DB障害は500でCookieを残す。削除済みトークンによる再試行も204とする。[API仕様](../docs/API仕様/post-api-logout.md)を参照する。

既存DBへは起動前に `make db-migrate` で版2のDELETE権限を適用する。API自身はmigrationを実行しない。画面からの操作と処理中・失敗時の表示は[画面仕様](../docs/画面仕様/ログイン・本人情報表示.md#ログアウトissue-54)を参照する。

```bash
make verify-logout-api
```

非公開の架空検証Userを再利用し、検証APIを空きポートで別起動する。対象セッションの削除・旧トークン401・再試行204・同じUserの他セッション／他Userの維持・CSRF拒否を確認する。さらに対象の検証行だけをロックし、実DB処理timeoutの500・Cookie保持と、ロック解除後の再試行を検証する。普段のAPIやDBは停止しない。

追加の架空User1件と検証セッションを作る。試験終了時はこの実行で発行したセッションだけをログアウトAPIで削除し、既存セッションは残す。追加Userと非公開の検証資格情報は保持し、次回も再利用する。秘密値は出力せず、資格情報ファイルと一時ログは.local配下・Git対象外とする。

## ログインAPIの実DB検証

```bash
make verify-login-api
```

`make verify-user-create` の非公開検証用Userを再利用し、別ポートに検証APIを起動する。未知ID・誤パスワード・CSRF拒否、NFC照合、Cookie属性、トークンとDBハッシュの対応、再起動後のDBセッション保持を確認する。1回に検証セッション2件を残す。値は表示せず、既存Userやセッションを削除しない。本人情報APIでの認証・同じCookieの期限更新・API再起動後の再認証も確認し、本人情報取得でDBのセッション行が変わらないことを検証する。

## Collection設定の取得（Issue #56）

`make db-migrate` で版3を適用した後、既存の架空Userへ設定を追加する。User作成は従来の非表示入力CLIで先に行う。

```bash
make seed-collections LOGIN_ID=fictional_user
```

seedはローカルDocker DBへの明示的な管理操作で、既存のmigration所有者を使用する。API実行用・User作成用の資格情報へ管理権限を与えない。対象Userから安定したfixture IDを作り、停止中Collection・Schedule・対象・情報源を1トランザクションで追加する。再実行で既存行や更新日時を変更せず、同じfixture IDに異なる内容があれば全体を中止する。既存設定の上書き・削除、Userの自動作成、外部情報取得は行わない。

`GET /api/collections` と `GET /api/collections/{ULID}` はCookie由来のUser IDで所有者を絞る。通常APIは設定テーブルをSELECTできるだけで、設定変更・DDLはできない。認証・取得が失敗したときは固定エラーと構造化ログを使い、生DBエラーや設定本文を出さない。[API仕様](../docs/API仕様/get-api-collections.md)参照。

verifyは2人の既存の架空検証Userを再利用する。新しい環境では、DB準備・migration後に以下の順で検証Userを用意する。通常の画面確認用Userとは別の検証用資格情報を `.local/` に保持するため、値を表示・公開しない。

```bash
make verify-user-create
make verify-logout-api
make verify-collections-api
```

Collectionのverifyは空一覧用の架空User1件を初回だけCLIで作る。2人へ専用seedを追加し、再実行不変・一覧/詳細・配下設定・空配列・401/400・他者/不存在の同一404・SELECT-only/作成者権限隔離・FK/LOOKBACK/一意性/ULIDを実DBで確認する。削除cascade試験はトランザクションをrollbackする。発行した試験セッションだけを最後にログアウトし、架空Userとseedを保持する。

## 架空の保存済みReport

既存Userを用意した上で、ルートから `make db-migrate`、`make seed-reports LOGIN_ID=fictional_user` を実行する。seedは専用の停止中Collection、成功Job/Attempt、Report本文と対象結果・事実・出典・対応・取得失敗を1トランザクションで保存する。AI・外部取得は行わない。既存Collection fixtureとは別の安定IDを使い、再実行は既存値を変更せず、衝突は全体をrollbackする。本文に含むHTML風の文字列も架空の表示検証用である。

`GET /api/reports` と `/api/reports/{ULID}` はCookie認証で本人の保存値だけを返す。ログイン後 `/reports` から詳細を開く。API権限はSELECTだけで、seedをAPI起動へ混ぜない。

`make verify-reports-api` は#53/#54の架空User fixtureがある環境で、空きポートAPIを起動し、再実行/衝突rollback、所有者認可、DB制約/権限、Collection削除後の履歴保持、ログ非混入を確認する。#53/#54 fixture未作成なら先に `make verify-user-create verify-logout-api` を実行する。検証専用の空Userと衝突確認Collection、削除後保持を確認する成功履歴一式は保持する。既存デモCollectionは削除しない。このコマンドは新しい検証履歴を毎回追加するため、大量の繰返し実行は避ける。

Report APIの実DB障害注入と#17全体の障害横断確認は #59、パスワード再設定は[管理コマンドの詳細](../docs/password-reset.md)を参照する。生成・Workerは #19、本格閲覧は #18で扱う。

保存Report seedはfixture namespace v3を使う。対象の業務状態は「重要な変化あり」、現在状態と更新事実の双方に同じ対象の出典を付け、本文・構造化事実・生成入力snapshotを一致させる。旧v1/v2の検証途中履歴が残る環境では、それらは以前の確認用記録であり、現行seedの参照例ではない。既存履歴は変更・削除せず、新規環境にはv3だけを追加する。`make verify-reports-api` はv3を明示して内容一致と全fixture行の再実行不変も確認する。

取得失敗の本文表示は正本のルールに従って情報源の名称・URLだけとし、reasonは構造化行に保持して本文へ含めない。

## パスワード再設定（Issue #58）

既存DBでは初回だけ `make db-provision-password-resetter`、続いて `make db-migrate` を明示実行する。新規DBの `make db-bootstrap` は再設定専用ロールも作る。既存ロール・資格情報は上書きしない。[初期設定と既存環境の追加手順](../docs/local-postgresql.md#再設定用の専用ロール追加issue-58)を参照する。

```bash
make password-reset LOGIN_ID=fictional_user
make verify-password-reset
```

IDを指定し、新パスワードを2回、端末で非表示入力する。共通Argon2idでハッシュ化してから専用ロールで接続し、User更新と全session削除を一体でcommitする。`RESET_DB_PASSWORD_FILE` はDBの秘密ファイルのパスだけを指定でき、既定は `../.local/watchbrief-password-resetter-password`。アプリのパスワードをMake変数・引数に入れない。

不一致・入力不正・対象なしでは変更しない。通信障害で結果不明の場合はDB状態を確認してから再実行する。処理中APIの完了可能性、ロックとPHC再読取による競合制御、再設定同士の順序、専用fixtureの保持範囲と試験の制限は[再設定の詳細](../docs/password-reset.md)にまとめる。

## Collection本体の新規保存（Issue #18）

migration版9を明示適用してAPI・Frontendを起動し、一覧の「新規作成」から停止中Collectionを保存する。名前・任意の調査方針・初回の確認期間を入力できる。配下設定の編集や有効化は後続範囲である。[API契約](../docs/API仕様/post-api-collections.md)と[画面仕様](../docs/画面仕様/コレクションの新規作成.md)を参照する。

`make verify-collection-creation` は、既存の `verify-issue17` の使い捨て環境へ新規保存の試験を追加する。通常DBは停止・初期化せず、専用project・空きポート・privateな資格情報を使い、最後にその環境だけを除去する。Go・Node、Frontend依存、Docker/Compose、goose、Chromiumの準備は[既存検証手順](../docs/verification-issue-17.md)と同じ。Chromiumに補助ライブラリが必要な環境はBROWSER_LIBRARY_PATHを指定する。

APIでは同じ要求の再送と8並列送信、別番号の並列作成・表示順、入力条件・CSRF・他User404、2回目の書込失敗による全rollbackと復旧再試行、削除後の旧要求拒否、最小権限を確認する。ブラウザでは直接URLからログイン復帰、入力エラーとフォーカス、PC/スマホ幅、実DB停止時の入力保持、実際の保存後に応答だけをブラウザで破棄した再試行、一覧・詳細・再読み込み、ログアウト時の下書き消去を確認する。応答破棄はブラウザfetch境界の注入であり、実ネットワーク切断やcommit応答喪失の試験とは区別する。

最初の要求が結果不明になった後は、同じ番号と入力で再試行する。画面離脱・再読み込み・認証状態変更で番号を失った場合は一覧で確認する。DBの要求記録は残るが、別の新規操作の同名作成を防ぐものではない。保存中にログアウトしても、認証通過済みのDB保存まで取り消す保証はない。

CIには通常の単体試験だけを含め、実DB・Chromiumはこの明示コマンドで確認する。実行結果・未確認はIssue/PRへ記録する。
