# ローカルの新規構築・一連の確認と障害復旧

現在のVue・Go・PostgreSQL実装を動かす入口。旧AI案の `local-development.md`・`verification-issue-53.md` とは区別する。通常DBのデータは保持し、起動へmigrationや初期設定を自動追加しない。

## 初めて起動する

Linux、Docker EngineとCompose、Python 3、Make、asdfを用意する。Go・Nodeはルートの `.tool-versions`、DBは `postgres:18.6-bookworm`、gooseは3.29.0を使う。

ルートで順に実行する。

```bash
make runtime-install
make frontend-install
make backend-deps
make tools-install
make db-prepare
make db-up
make db-bootstrap
make db-migrate
make db-migration-status
make user-create LOGIN_ID=fictional_user
make seed-collections LOGIN_ID=fictional_user
make seed-reports LOGIN_ID=fictional_user
```

`db-bootstrap` は空の新規環境で1回だけ。既存ロールを上書きしない。初期管理・API・作成者・migration・再設定の秘密はGit対象外 `.local/` の600ファイルに分離され、値を表示しない。アプリのパスワードは管理CLIの端末へ2回非表示入力し、引数・Make変数・パイプへ渡さない。架空専用パスワードを使う。seedは外部へ接続せず停止中設定と保存履歴を原子的に追加し、再実行で既存値を上書きしない。実際の設定や本文をfixtureへ流用しない。

別々の端末で `make backend-dev` と `make frontend-dev` を起動する。`http://127.0.0.1:5173/reports` を開き、ログイン後に同じ画面へ戻ること、本人情報、自分の設定とReport詳細を確認する。本文中のHTMLはテキストとして表示する。「ログアウト」で入力・個人データが消え、再読み込みでもログインフォームになる。管理者による再設定は `make password-reset LOGIN_ID=fictional_user`。対象Userの全セッションを失効する。

停止は各アプリ端末でCtrl+C、DBは `make db-stop`。再開は `make db-up` とアプリ起動だけでよく、bootstrapを繰り返さない。DB停止でデータは消えない。既存の3ロール環境を更新する場合だけ[再設定ロールの追加手順](local-postgresql.md#再設定用の専用ロール追加issue-58)を使う。問題があってもreset・volume削除・Down・版管理行の手編集を復旧手順にしない。

## 通常環境を止めずに検証する

```bash
make check
make build
make sqlc-vet
make goose-validate
make verify-issue17
```

`verify-issue17` は上記依存導入とgoose、Chromiumが必要。OSにChromiumがなければ `CHROMIUM_BIN` で実行ファイルを指定する。既存のPlaywright Chromiumキャッシュも検索する。共有ライブラリの補助配置が必要な環境だけ `BROWSER_LIBRARY_PATH` で指定できる。これはブラウザ試験環境の設定であり、アプリの本番依存ではない。Chromiumは専用プロファイルでheadless起動する。sandboxを無効にするため、管理されたローカル検証環境だけで実行する。

runnerはprivateな一時ディレクトリへ現在のスクリプト・Composeをそのまま配置し、現在のbootstrap SQL・migration・seedを使う。SQLの検証用複製は持たない。ランダムな専用Compose projectと空きloopbackポート（5432は拒否）に新しいDBを作る。通常project `watchbrief-local`・通常 `.local/` の秘密・既存volumeには触れない。APIとViteも専用空きポートを使う。終了時は自分のprocess・専用projectのvolume・一時状態だけを削除する。強制終了などで残った場合は、対象projectが当該runnerの `watchbrief-verify-` 名であることを確認してから対処し、通常DBへ削除コマンドを使わない。

| 確認する経路 | 成功・失敗の判定 |
|---|---|
| 新規構築 | 4用途別roleのbootstrap、goose1〜8、再適用の安全性、通常roleのDDL・role管理拒否、制約試験 |
| API | CLI作成、本人・設定・Report、他Userと存在しないIDの同じ404、CSRF拒否、logout後の旧token401 |
| 再設定 | 2旧token失効、旧password拒否、新password成功、他Userのsession維持 |
| 実DB停止 | login/me/設定・Report一覧と詳細/logoutが500、Cookie不変更、reset非成功。復旧後にUser/hash/session不変を照合し、手動再操作で成功 |
| ブラウザ | 未認証のReport詳細→login→元のReport、外部戻り先の安全な既定、本人・設定・本文テキスト、実DB停止時の安全な表示、個人データ消去、復旧再試行、logout失敗時のCookie維持と再試行後401 |
| ログとエラー | 構造化ERRORのtime/msg/operation/stage/result/version、password・token・hash・DB秘密・Authorization・接続文字列・個人設定/本文の非混入。秘密値・本文を試験出力へ表示しない |

APIには既存の分類済みラベルを残す。実DB停止中の設定・Reportでは認証照合の段階で失敗する場合がある。ブラウザの設定/Report試験では実際に成功した本人情報応答の受け渡しを待機し、専用DBを停止してから応答を解放し、その後の取得SQLを失敗させる。HTTPステータスや本文を偽造しない。全SQLの失敗パターンを網羅したものではない。CLIの失敗案内は内部情報を含まず、結果不明の場合はDB状態確認を案内する。ログイン試行や利用者ごとの専用イベント基盤は追加しない。

## 確認状況の記録と制限

実行結果はIssue/PRへ「実際に動作確認済み」「設定・文書のみ確認」「未確認・省略」に分けて記録する。この文書に手順があることを実動作の証明にしない。各子Issueの既存試験は[Backend](../backend/README.md)・[再設定](password-reset.md#検証の入口と範囲)を参照する。競合の実DB試験・timeout試験と、本runnerの実停止による切断/接続拒否を区別する。

本runnerは全故障種類を保証しない。commit応答喪失、DB処理途中のOS強制終了、ネットワークblackhole、ディスク障害、大量負荷、実TLS/HTTPS・nginx・AWS・外部AI接続は未確認範囲。検証用DBは使い捨てであり、実環境のバックアップ/復元試験ではない。時間・無操作によるセッション失効は合意どおり追加しない。利用者向け端末管理は今回の対象外。

#18は設定編集と本格的な閲覧、#19はScheduler/Workerと生成処理、#49はログイン試行制限・認証専用記録へ引き継ぐ。公開は承認・検証していない。公開・実利用を判断する際は、試行制限と認証試行追跡が弱い制限を構成とともに再確認する。現在の画面/API契約は[画面仕様](画面仕様/README.md)・[API仕様](API仕様/README.md)、論理的な責務と公開境界は[システム設計](system-design.md)を参照する。

## Collection作成の追加検証（Issue #18）

`make verify-collection-creation` は同じ隔離runnerを使い、新規作成・安全な再試行のAPI/ブラウザ検証を追加する。現在のrunnerはgooseディレクトリの最新番号まで明示migrationが適用されたことを確認する。通常の `verify-issue17` は従来の認証・取得・復旧確認を維持する。

範囲、ブラウザでの応答破棄の位置、要求番号を失った後の制限は[Backendの作成検証](../backend/README.md#collection本体の新規保存issue-18)を参照する。下書きのログアウト消去・遅延応答破棄はVitestでも確認し、実ブラウザ・実DBと区別する。
