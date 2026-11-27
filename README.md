# WatchBrief

WatchBriefは、気になる企業・人物・AI・技術などの公開情報を定期的に調査し、Markdownレポートとして蓄積・閲覧する個人向けWebアプリです。

情報を集めるだけでなく、出典を残し、「現在の状態」「更新・新着情報」「AIによる整理」を分けて読める状態を目指します。

## 何のためのものか

WatchBriefは、継続して追いかけたい公開情報をコレクションごとにまとめる仕組みです。設定したスケジュールで非同期に調査し、保存済みレポートを認証付きWeb画面から日付別・コレクション別に閲覧します。

扱う情報は公開情報を前提にします。実際の保有銘柄、資産情報、個人的なウォッチ設定、APIキー、ログ、生成キャッシュはこのリポジトリでは管理しません。

## 想定している使い方

MVPでは、次の使い方を想定しています。

1. ウォッチ対象を設定する
2. 設定したスケジュールで公開情報を自動収集する
3. 引用付きMarkdownレポートを生成する
4. 日付別のレポート履歴として保存する
5. Webアプリで読む

レポートでは、現在の状態、前回成功以降の更新・新着情報、事実、出典、AIによる整理、取得できなかった情報源を分けて残します。

## 現在できること

ローカルで、管理CLIによる架空User作成・パスワード再設定、ログイン・本人情報表示、自分のCollection設定と保存済みReportの取得、ログアウトまでつながっています。保護URLはログイン後に安全な戻り先へ復帰します。既存設定とReportの取得に加え、停止中Collection本体の新規保存と同じ要求の再試行に対応します。停止中Collection本体の編集と競合検出・安全な再試行にも対応します。停止中Collectionの対象・情報源・Schedule追加/編集を同じ画面で一括保存できます。有効化・削除・並べ替え等は後続範囲で、Report本文は安全なMarkdown表示に対応します。

最初は[新規環境の起動・確認手順](docs/verification-issue-17.md)に従ってください。`make verify-issue17` は通常DBを止めず、専用の使い捨てDBで新規構築・実DB障害と復旧・ブラウザ操作を検証します。

設定編集の残りと本格的な閲覧は #18、Scheduler・Workerによる生成は #19、外部接続は #20、AWS配備は #21、ログイン試行制限・認証専用記録は #49の後続作業です。現在の架空seedは生成処理ではありません。旧AI案の文書は参考として区別し、現在の契約は[画面仕様](docs/画面仕様/README.md)・[API仕様](docs/API仕様/README.md)を参照します。

## ドキュメント

- [ドキュメント一覧](docs/README.md)
- [プロダクト仕様](docs/product-definition.md)
- [画面仕様](docs/画面仕様/README.md)
- [API仕様](docs/API仕様/README.md)
- [システム設計](docs/system-design.md)
- [ドメインモデル](docs/domain-model.md)
- [DB設計・ER図](docs/database-design.md)
- [技術選定の前提](docs/technical-direction.md)
- [作業計画と将来候補](docs/work-plan.md)

## 公開上の注意

このリポジトリはPublicです。ここに置いたソースコードとドキュメントは公開情報として扱います。将来デプロイするWebアプリの公開範囲と認証設定は、リポジトリの公開設定とは分けて管理します。

個人情報、APIキー、実際の資産情報、公開する意図のないウォッチ設定、ログ、生成済みReportは含めないでください。

## ローカルで見る

[新規環境の起動・確認手順](docs/verification-issue-17.md)を入口に、[Frontend](frontend/README.md)・[Backend](backend/README.md)・[ローカルPostgreSQL](docs/local-postgresql.md)で詳細を確認できます。旧AI案の `docs/local-development.md` は現在の起動手順として使いません。

```bash
git clone https://github.com/smofktn/watchbrief.git
cd watchbrief
```

## 開発コマンド

リポジトリルートで `make help` を実行すると一覧を確認できます。Frontend・Backend内でも `make help` が使えます。DB操作はBackendのMakefileが担当し、ルートから同名ターゲットへ委譲します。

初期設定・依存導入・明示migrationが済んだ環境では、次でDBコンテナ・API・画面をまとめて起動できます。

```bash
make dev
```

DBの起動完了を待ち、既存の `backend-dev` と `frontend-dev` を起動します。画面は `http://127.0.0.1:5173`。終了はCtrl+Cで、API・画面とその子プロセスを終了します。片方が異常終了した場合も、もう一方を終了して失敗を返します。DBは残るので、停止するときは `make db-stop` を使ってください。既に別の端末で起動しているアプリは自動終了せず、ポート競合は起動エラーになります。

`make dev` は初期設定・依存導入・migrationを自動実行しません。初回は[構築手順](docs/verification-issue-17.md#初めて起動する)、DB更新時は `make db-up` → `make db-migrate` → `make dev` の順に実行してください。個別起動には、別々の端末で従来どおり `make backend-dev` / `make frontend-dev` も使えます。

架空Userを追加する場合は、DBの初期設定・migration後に実行します。パスワードは端末で2回非表示入力します。

```bash
make user-create LOGIN_ID=fictional_user
```

| 操作 | ルート | Frontend内 | Backend内 |
|---|---|---|---|
| 起動 | `make dev`（一括）、`make frontend-dev` / `make backend-dev`（個別） | `make dev` | `make dev` |
| ビルド | `make build` | `make build` | `make build` |
| 試験・型検査 | `make check` | `make test` / `make typecheck` | `make test` |
| Backend coverage | `make backend-coverage` | — | `make coverage` |
| 両側のcoverage | `make coverage` | — | — |
| Frontend coverage | `make frontend-coverage` | `make coverage` | — |
| DB起動・状況確認 | `make db-up` / `make db-status` | — | 同名ターゲット |
| migration | `make db-migrate` | — | `make db-migrate` |
| User追加 | `make user-create LOGIN_ID=fictional_user` | — | 同名ターゲット |

依存導入は `make frontend-install`、goose/sqlc導入は `make tools-install`、asdfでのGo/Node導入は `make runtime-install` を使います。初回のDB準備と各試験の前提は[ローカルPostgreSQL](docs/local-postgresql.md)・[Backend](backend/README.md)を参照してください。初期設定やmigrationは通常起動へ自動追加しません。

## 開発について

実装上の注意、運用保守ルール、未決定事項の扱いは[開発・運用ルール](CONTRIBUTING.md)を参照してください。

main向けPRではGitHub Actionsが既存の `make check`・`make build` を自動実行します。対象範囲、ローカルでの再現コマンド、失敗時の確認方法は[CI手順](CONTRIBUTING.md#ciissue-70)を参照してください。

保存済みReportの最小経路は `make db-migrate` → `make seed-reports LOGIN_ID=fictional_user` → ログイン → `/reports`。架空Userを先に作成する。`make verify-reports-api` の前提・保持する架空データ・制限は[Backend手順](backend/README.md#架空の保存済みreport)、契約は[Report API](docs/API仕様/get-api-reports.md)と[画面仕様](docs/画面仕様/保存済みReportの取得.md)を参照する。

パスワード再設定は `make password-reset LOGIN_ID=fictional_user` で行います。初期設定・既存DBへの専用ロール追加と試験は[再設定の詳細](docs/password-reset.md)を参照してください。

Collection本体の新規作成は一覧の「新規作成」から操作します。事前に版9の明示migrationが必要です。通常DBを停止しない追加検証は `make verify-collection-creation`。範囲と制限は[Backend手順](backend/README.md#collection本体の新規保存issue-18)を参照してください。

Collection本体の編集は版10の明示migration後、停止中の詳細から操作します。事前の隔離検証は `make verify-collection-update`。[操作・検証手順](backend/README.md#collection本体の編集issue-74)を参照してください。

停止中Collection本体とWatchTargetの追加・編集をまとめて保存する手順は[Frontend](frontend/README.md#collectionと対象の一括編集issue-78)、版11の明示migrationと隔離検証 `make verify-collection-targets` は[Backend](backend/README.md#本体とwatchtargetの一括保存issue-78)を参照してください。通常起動・#21の配備への自動migrationは行いません。

### 情報源の一括保存（Issue #80）

停止中Collection詳細の編集で、保存済み対象の情報源を追加・編集し、本体/対象とまとめて保存する。新規対象には保存後に情報源を追加する。URLは検証・保存・テキスト表示だけを行い、外部アクセスや正規化・重複検査・削除は行わない。契約は[更新API](docs/API仕様/patch-api-collections.md)を参照。

検証入口はルートの `make verify-collection-sources`。使い捨てCompose project/架空データでmigration 9→10→11→12と再適用、旧要求digest/再送、原子的保存・所属/CSRF・URL/上限・競合/rollback・履歴不変、Chromiumを検証する。goose版12は情報源の必要列権限のみ追加し、通常DB/本番/EC2には適用しない。通常起動/配備への自動migrationも追加しない。

### Scheduleの一括保存（Issue #82）

停止中Collectionの編集で、日本時間のraw cron式を追加・編集し、本体・対象・情報源とまとめて保存します。例は `0 7 * * *`（毎日7時）。未設定の空欄は追加せず、登録済みの式を空にして削除することはできません。保存で生成は開始しません。[画面/API契約](docs/画面仕様/コレクション本体の編集.md#schedule追加編集issue-82)を参照してください。

通常DBへの自動適用はありません。版13はScheduleの必要列権限だけを追加します。専用Compose project/架空データの検証入口は `make verify-collection-schedules`。[検証手順](backend/README.md#scheduleの一括保存issue-82)を参照してください。

### 保存済みReportのMarkdown表示（Issue #84）

所有するReportの詳細で保存済み本文を見出し・段落・強調・リスト・コード・安全な出典リンクとして表示します。生HTMLや画像は文字として表示し、整形不可時も安全なテキストへ戻します。API/保存本文は変更せず、閲覧時に生成・外部取得を行いません。[画面契約](docs/画面仕様/保存済みReportの取得.md#安全なmarkdown表示)を参照してください。

検証入口は `make verify-report-markdown`。通常DBを操作しない使い捨て環境でChromiumのネットワーク監視も行います。[検証と依存の手順](frontend/README.md#保存済みreportのmarkdown表示issue-84)を参照してください。

### 保存済みReport一覧の追加取得（Issue #86）

`/reports` は新しい順に30件まで表示し、「もっと読む」で30件ずつ追加します。続きの失敗は表示済み項目を残して再試行でき、一覧から離れる/再読み込みすると最初から表示します。新しい先頭分は再読み込みで確認します。これは成功Reportの最小一覧で、日付/Collection別・待機/実行/失敗・homeは後続です。

検証入口は `make verify-report-pages`、手動の架空35件seedは `make seed-report-pages LOGIN_ID=fictional_user`。本作業では通常DBへseedを行わず、専用の使い捨て環境だけで検証します。[API契約](docs/API仕様/get-api-reports.md#30件ずつのcursor追加取得)・[検証手順](backend/README.md#保存済みreport一覧の追加取得issue-86)を参照してください。

## 日付別の保存済み予定・結果（Issue #88）

`/reports` 内のリンクから `/reports/daily` を開き、日本時間の日付ごとに保存済みJobの4状態を30件ずつ読む。成功だけ既存Report詳細へ移動する。手動更新は先頭30件へ戻る。生成処理は実行しない。

検証入口はルートの `make verify-daily-jobs`。使い捨てDB/架空データ/Chromiumによる範囲と事前条件は[Backend手順](backend/README.md#日付別の保存済み予定結果issue-88)を参照する。

契約は[Job一覧API](docs/API仕様/get-api-generation-jobs.md)と[日付別画面](docs/画面仕様/日付別の予定・結果一覧.md)。検証結果はIssue/PRに記録する。
