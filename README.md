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

ローカルで、管理CLIによる架空User作成・パスワード再設定、ログイン・本人情報表示、自分のCollection設定と保存済みReportの取得、ログアウトまでつながっています。保護URLはログイン後に安全な戻り先へ復帰します。既存設定とReportの取得に加え、停止中Collection本体の新規保存と同じ要求の再試行に対応します。対象・Schedule等の編集は後続範囲で、Report本文はテキスト表示です。

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

```bash
make frontend-dev
```

別の端末でAPIを起動します。

```bash
make backend-dev
```

架空Userを追加する場合は、DBの初期設定・migration後に実行します。パスワードは端末で2回非表示入力します。

```bash
make user-create LOGIN_ID=fictional_user
```

| 操作 | ルート | Frontend内 | Backend内 |
|---|---|---|---|
| 起動 | `make frontend-dev` / `make backend-dev` | `make dev` | `make dev` |
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
