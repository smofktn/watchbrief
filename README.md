# WatchBrief

WatchBriefは、気になる企業・人物・AI・技術などの公開情報を定期的に調査し、Markdownレポートとして蓄積・閲覧する個人向けWebアプリです。

情報を集めるだけでなく、出典を残し、「現在の状態」「更新・新着情報」「AIによる整理」を分けて読める状態を目指します。

> [!IMPORTANT]
> Issue #53は、設計を利用者とAIで検討し、合意した範囲の実装・検証をAIが担当する形で進めています。以前のAIによる未完成の実装案を参考として保持していますが、採用済み・利用可能・検証完了とは扱いません。

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

Vue・TypeScript・Vite・PrimeVueによる表示確認用の最小画面があります。[Frontendの起動手順](frontend/README.md)で確認できます。[Go API](backend/README.md)は準備済みのローカルDBへ接続して起動します。画面からVite経由でAPIへ入力を送信できます。[現在の管理コマンド](backend/README.md#架空userの作成)で架空UserをDBへ追加できます。ログイン認証・セッション保存・Cookie発行は接続済み。Cookieから本人情報を取得し、ログインIDを表示できます。画面を再読み込みしてもCookieで再認証します。 [ログアウトAPI](docs/API仕様/post-api-logout.md)は、対象セッションの削除・旧トークン拒否まで実装しています。本人情報画面のボタンから直接ログアウトでき、開始時に本人情報を消して、成功時はログインフォーム、失敗時は再試行を表示します。

架空Userの管理コマンド、Vueログイン画面、Goのログイン・本人情報API、認証DB migrationの既存AI案があります。旧案は現在の実装と区別します。現在は架空User作成からログイン・本人情報表示までの最小経路を実装しています。[過去のAI担当による検証記録と制限](docs/verification-issue-53.md)を確認してください。

初期構成はEC2 1台にnginx・アプリ・PostgreSQLを同居させ、Ubuntu Server LTSとsystemdを使い、手動でデプロイする方針です。最小骨格にはVue・Go、DBアクセスにはSQL＋sqlcを使います。[既存AI案の具体方式](docs/auth-implementation.md)は参考であり、それだけで採用決定にはしません。配置・運用の合意事項と設計案は[システム設計](docs/system-design.md)にまとめています。

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

現在の最小骨格は[Frontend](frontend/README.md)・[Backend](backend/README.md)の手順で起動できます。DB単体の起動は[ローカルPostgreSQL](docs/local-postgresql.md)を参照してください。ログインにはDBの初期設定・migrationと架空User作成が必要です。

[既存AI案のローカル起動・検証手順](docs/local-development.md)は参考として残しており、全手順の実行成功は未確認です。ファイルを確認するだけなら、通常のGit cloneで取得できます。

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
| 試験・型検査 | `make check` | `make typecheck` | `make test` |
| DB起動・状況確認 | `make db-up` / `make db-status` | — | 同名ターゲット |
| migration | `make db-migrate` | — | `make db-migrate` |
| User追加 | `make user-create LOGIN_ID=fictional_user` | — | 同名ターゲット |

依存導入は `make frontend-install`、goose/sqlc導入は `make tools-install`、asdfでのGo/Node導入は `make runtime-install` を使います。初回のDB準備と各試験の前提は[ローカルPostgreSQL](docs/local-postgresql.md)・[Backend](backend/README.md)を参照してください。初期設定やmigrationは通常起動へ自動追加しません。

## 開発について

実装上の注意、運用保守ルール、未決定事項の扱いは[開発・運用ルール](CONTRIBUTING.md)を参照してください。

保存済みReportの最小経路は `make db-migrate` → `make seed-reports LOGIN_ID=fictional_user` → ログイン → `/reports`。架空Userを先に作成する。`make verify-reports-api` の前提・保持する架空データ・制限は[Backend手順](backend/README.md#架空の保存済みreport)、契約は[Report API](docs/API仕様/get-api-reports.md)と[画面仕様](docs/画面仕様/保存済みReportの取得.md)を参照する。

パスワード再設定は `make password-reset LOGIN_ID=fictional_user` で行います。初期設定・既存DBへの専用ロール追加と試験は[再設定の詳細](docs/password-reset.md)を参照してください。
