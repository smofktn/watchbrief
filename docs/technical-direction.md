# WatchBrief 技術選定の前提

> **Issue #53の既存案について**：以前のAIが作成した未完成の実装案と、それに伴う文書変更を参考として保持している。具体バージョン、ハッシュ、Cookie・CSRF、migration等は、今回利用者と確認したもの以外は採用済みと扱わない。過去の検証結果も今回の実動作確認とは区別する。現在の進め方は[作業計画](work-plan.md#利用者主体で子issueを進める運用合意済み)を参照する。

> [!NOTE]
> この文書は、技術を選ぶときの目的、重視する性質、採用済みの方針と現在の候補を整理する。候補は採用決定ではない。決定した内容は、理由と影響を記録した上でシステム設計や実装文書へ反映する。

## プロジェクトの目的

WatchBriefの主目的は、個人向けの情報整理Webアプリを作ることである。補助目的として、AWSを使ったサーバー構築と運用を学ぶ。

学習目的であっても、最初から構成を過度に複雑にしない。実際のアプリに意味があり、情報や利用事例が多く、今後にも応用しやすい技術を優先する。

初期にSSHを使う背景には、将来[media-vault](https://github.com/smofktn/media-vault)でRaspberry Piをサーバーとして使う際にも、SSHやLinuxの運用経験を活かしたいという利用者の意向がある。Raspberry Piでの構築は現時点では仮の予定であり、WatchBriefの配備先を変更するものではない。他リポジトリの構成・規約をWatchBriefへ流用する決定でもない。

## 技術選定で重視すること

- 初心者でも構築、障害調査、復旧の流れを理解できること
- 小さく開始し、必要になった段階で拡張できること
- エラーハンドリングと入力検証を明示的に実装できること
- 非同期処理、timeout、部分失敗、重複実行、復旧を扱えること
- DB設計とドメイン設計を実装都合だけで崩さないこと
- 構造化ログと監視により、画面、API、Job、Attemptを追跡できること
- 性能を推測だけで最適化せず、代表的な処理を計測できること
- 認証情報や利用者データをPublicリポジトリへ含めないこと

過度なCQRSやイベントソーシングは初期候補にせず、必要十分な責務分離を目指す。Issue #53のBackendでは、ドメイン・ユースケースへ依存を向ける小さなクリーンアーキテクチャを採用する。HTTP・CLI・DB等の具体実装を外側に置き、必要なインターフェースだけを使う。[責務と依存方向](system-design.md#go内部の依存方向合意済み)を参照する。全面的なDDD実装や、すべての型の抽象化を採用する決定ではない。具体パッケージ・frameworkは必要な段階で確認する。

## 採用済みの方針と現在の候補

| 領域 | 現在の候補・意向 | 状態 |
|---|---|---|
| リポジトリ | Web、worker、共通ドメイン、インフラを同じリポジトリで扱うmonorepo | 有力候補。workspace構成は未決定 |
| 初期インフラ | 画面配信、API、Scheduler、Worker、停止監視、DBをEC2 1台へ同居。保守・障害時の利用停止を許容する | 採用。Issue #11で初期からRDSを使う決定を更新。構築・運用を学びながら段階的にAWSへ任せる |
| 初期OS | 標準版のUbuntu Server LTS | 採用。Amazon Linux 2023と比較し、ほかのLinuxサーバーやRaspberry Piにも経験を活かしやすいことを重視。具体バージョン、AMI、CPUアーキテクチャ、更新・再起動手順は未決定 |
| 初期リージョン | 東京（`ap-northeast-1`）にEC2と専用VPCを配置する | 採用。日本からの利用を想定し、近い地域へまとめる。外部AIサービスの利用地域は別途確認 |
| 初期ネットワーク | WatchBrief専用VPCを新規作成し、パブリックサブネットにEC2を置き、Internet Gateway経由で接続する | 採用。デフォルトVPCと分けて管理し、経路・接続制限の構築を学ぶ。AZ、IP範囲、IPv6の採用範囲、作成手順・IaC方式は未決定 |
| 公開IPv4 | Elastic IPを1つ使用する | 採用。EC2停止・起動時もドメインの接続先を維持する。保持中の課金と不要時の解放を管理する |
| 独自ドメイン | 取得して利用するが、具体的な決定は後回しにする。アプリ別の取得と共用親ドメインの利用を比較候補として残す | 共用方針を未決定へ更新。名前の候補にsmofktnが挙がったが、完全な名前、登録事業者、DNS、予算、更新・登録者情報の扱いは未決定。購入は別途確認 |
| アプリの分割 | FrontendとAPIを分離し、EC2上ではAPI、Scheduler、Worker、停止監視を別プロセスで動かす | 採用 |
| Web公開の入口 | 同じEC2のnginxが画面ファイルを配信し、ブラウザのJavaScriptからのAPI要求を中継する | 採用。同じサイトURLで利用する。ドメイン、HTTPS、内部ポートなどの具体設定は未決定 |
| 初期API・DBの接続範囲 | APIプログラムとPostgreSQLはEC2内部からのみ接続可能とし、待受設定とSecurity Groupで外部からの直接接続を許可しない | 採用。具体的なアドレス・ポート・ソケット方式、認証、管理用接続設定は未決定 |
| EC2の管理接続 | 初期は管理回線のグローバルIPに限定したSSH鍵認証。パスワードによるSSHログインは無効化。将来はSession Managerへ早めに移行 | 採用。移行優先度はDBのRDS分離の次くらいが目安。実際のIP、鍵の具体仕様・管理、OSユーザー・権限、移行時期・手順は未決定 |
| 初期の起動管理 | systemdで各プロセスを管理する | 採用。具体設定は未決定 |
| 初期デプロイ | 運用者が更新を開始する手動デプロイ | 採用。配置・再起動・確認・切り戻しの流れを学び、後の自動化を見据える。スクリプト化、ビルド場所、転送、DB変更対応、自動化方式・時期は未決定 |
| 将来のインフラ | DBをRDSへ分離し、アプリをコンテナ化してECSなどで動かすことを目指す | 将来の方向性として合意。移行時期・条件・具体的な実施順、ECSの採用確定とFargateなどの実行方式は未決定 |
| Frontend | Vue 3・TypeScript・Vite | 利用者と採用に合意。Vueで画面を構成し、TypeScriptで型を検査し、Viteで開発・ビルドする。旧AI案のJavaScript module・独自ビルドは今回の採用構成としない。Node.jsは26.13.0を固定し、依存の具体バージョンと互換性は導入時に確認する |
| UIコンポーネント | PrimeVue | 利用者と採用に合意。Vue 3・TypeScript対応の部品を組み合わせる。標準テーマから始める案で、具体テーマ・版・ライセンス条件は導入時に確認する。有料テンプレート・サポートの購入は含まない |
| Frontend状態管理 | TanStack Query | サーバー状態管理の候補。自動retry・再取得・cache既定値はそのまま採用しない |
| Client store | Piniaは初期候補にしない | 必要性が生じた場合に再検討する |
| Backend | Go。開発用の版は1.27.3 | Go採用とasdfでの版管理に合意。HTTP framework・具体ドライバーは必要な段階で確認する。旧AI案のnet/http等は今回の採用決定と区別する |
| DBアクセス | SQL＋sqlc | 利用者と方針に合意。pgx/v5等のドライバーと具体版は実装前に確認する。旧AI案の比較・版は[参考の実装方式](auth-implementation.md#最小構成と選定)に保持する |
| 初期認証 | アプリのID・パスワード認証とPostgreSQLで管理するサーバー側セッション。Cookieでランダムトークンを渡す | Issue #1で採用。ログアウト時にサーバー側で無効化でき、既存DBを使えることを重視。Basic認証・自己完結型JWTは見送り。Cookieの保護属性・Go標準CSRF防御・Origin完全一致検証は[API共通仕様](API仕様/README.md#cookieの保護とcsrf合意済み)に合意。保存更新等の未決定事項は実装前に具体化。旧AI案とは区別する |
| パスワードハッシュ | Argon2id | 利用者と方式に合意。ランダムsaltを使い、管理コマンドとAPIで同じ処理を共有する。[設計](system-design.md#共通パスワードハッシュ合意済み)に従い、入力条件は[プロダクト仕様](product-definition.md#ユーザーと認証)の合意に従い、具体パラメーター・ライブラリ・保存形式は別途具体化する |
| ユーザー作成・パスワード再設定 | 共通ハッシュ処理を使う管理コマンド。再設定では対象Userの全既存セッションを無効化 | Issue #1で再設定時の失効に合意。パスワード更新と失効は同じDBトランザクションで確定。User作成・共通hashの詳細は[Issue #53の基本設計](system-design.md#issue-53の基本設計)から具体化。再設定・競合制御は #58へ |
| ログイン失敗制限・認証専用記録 | 初期MVPでは作り込まず後続改善 #49 へ回す | 利用者の指示を反映。通常の認証・認可・入力検証と既存のエラーログ・秘密情報非出力は維持。具体的な制限・イベント記録・専用監査機能は未採用 |
| 通常実行と管理操作の権限 | アプリの非root実行、DB通常実行用と管理用の権限分離、AWS APIが必要な場合の最小権限IAMロール | #53では[通常API・User作成・migrationの用途別分担](database-design.md#用途別のdb権限合意済み)にも合意。Issue #1で大枠に合意。通常のアプリへOS・DB・AWSの管理権限を渡さない。ユーザー・ロール名、プロセス別の配分、具体設定・試験は実装・構築へ |
| Secretの保管 | 初期のEC2保護ファイル案と、Secrets Managerへの早期移行 | 利用者の意向を記録。具体方式・移行時期・費用は未決定。取得処理をアプリの接続処理から分ける構成は提案。Parameter Storeの中間導入やAWS作成・課金は決定していない |
| migration | gooseによるversioned SQL migration | 利用者と採用に合意。SQLの適用・適用済みバージョン管理をgooseへ任せ、API起動とは別に管理用権限で明示実行する。旧AI案の独自psqlスクリプトは今回の採用方式ではない |
| DBの配置 | 初期はアプリと同じEC2、将来はRDSへ分離 | 採用。自前運用とマネージドサービスの違いを段階的に学ぶ |
| RDSの公開境界 | インターネットへ公開せず、通常のDB接続元をアプリ実行環境に限定する | 将来RDSを使う場合の方針として合意。具体設定と管理用接続経路は未決定 |
| 初期DB製品 | PostgreSQL | 採用。複数プロセスでの利用、自前DBの運用学習、将来同じエンジンでRDSへ移せることを重視。SQLiteは初期の管理が簡単だが、今回の目的から見送る |
| 初期DBストレージ | OS用とは別のDB用EBSへPostgreSQLのデータを保存し、EC2削除時も保持（`DeleteOnTermination=false`） | 採用。保持中は課金対象。容量、種類、保存場所、マウント・権限、保持後の管理・削除手順とOS用EBSの削除方針は未決定。バックアップではない |
| EBS暗号化 | OS用・DB用とも東京リージョンのAWS管理KMS鍵 `aws/ebs` で暗号化する | 採用。初期にはカスタマー管理鍵を作らず管理負担を抑える。構築時に暗号化状態と使用鍵を確認する |
| DBの具体構成 | PostgreSQLのバージョン、将来のRDS移行先などを比較する | バージョン、将来の移行先エンジン（Auroraを含む比較）、サイズ、可用性構成、バックアップ設定、料金見積もりは未決定 |
| AI | Amazon Bedrock、OpenAI API、または併用 | 未決定。BedrockはAWS学習との相性から候補 |
| 情報収集 | AIサービスとは別のWeb検索・取得手段 | 未決定 |
| 外部URL取得のSSRF対策 | 初期MVPで内部向け接続を拒否し、名前解決後の実接続先・転送先にも適用 | Issue #1で大枠に合意。具体アドレス判定・HTTPクライアント・外部サービス側の制御範囲と試験は #20 へ |
| 外部サイトの危険判定 | 悪意あるサイトを拒否する仕組みを将来導入したい意向 | #50で追跡。初期MVPへ追加しない。危険性の定義・方式・サービス・料金は未決定。初期SSRF方針とは別に扱う |
| IaC | ツールと導入時期を含め未決定 | システム設計に再現可能な構築記録と導入時期の比較案を記載。ツール選定は構築作業へ引き継ぐ |

Issue #53の現在の構成と検討中の画面・APIは[基本設計案](system-design.md#issue-53の基本設計)を参照する。旧AI案の具体バージョン・パスワードハッシュ・Cookie・CSRF・配置と比較理由は[参考の実装方式](auth-implementation.md)に保持する。旧案だけを理由に採用しない。Router・TanStack Query・queue等は必要性を確認してから判断する。

Frontendの一次情報は[VueのTypeScript対応](https://vuejs.org/guide/typescript/overview.html)、[Vite](https://vite.dev/guide/)、[PrimeVueのソース・ライセンス](https://github.com/primefaces/primevue)を参照する。UIライブラリは画面部品を担当し、認証・認可・CSRF・APIの入力検証を代替しない。

PrimeVueの採用合意は、最新メジャー版の利用条件への同意を意味しない。npm配布物のPrimeVue 5.0.4と `@primeuix/themes` 3.1.0は、従来のMIT版とは異なるPrimeUI Licenseを記載し、ライセンスキー・Community Licenseの適格条件と年次更新を求めている。これらの版は未導入・未採用とする。MIT版の4系と対応テーマの互換性・保守状況を確認する案と、5系の利用条件を確認する案を利用者と相談し、導入する版を決める。配布物：[PrimeVue 5.0.4](https://registry.npmjs.org/primevue/-/primevue-5.0.4.tgz)、[themes 3.1.0](https://registry.npmjs.org/@primeuix/themes/-/themes-3.1.0.tgz)。旧GitHubリポジトリのMIT表記だけで最新版の条件を判断しない。

### migrationツール（goose採用）

gooseを使い、`backend/db/migrations/000001_create_auth.sql` 等の番号付きSQLでスキーマ変更を管理する。ファイル名は配置例であり、実装済みという意味ではない。1ファイルに `-- +goose Up` と `-- +goose Down` を記述し、適用・適用状況の管理をツールへ任せる。Goでスキーマ変更を書く方式やORMの自動migrationは今回の採用に含めない。

sqlcはGoのDBアクセスコードを生成し、gooseは実DBへSQLを適用する。sqlcはgoose形式のmigrationを読み、Down側を無視できる。役割を分け、API起動時にはmigrationを実行せず、通常APIとは別の管理用DB権限で明示実行する。旧AI案の独自psqlによる版管理を採用・完成させる決定ではない。

golang-migrateもSQLベースの代替候補として比較したが、Up/Downを同じファイルで確認でき、sqlcも形式を読めるgooseを採用した。代わりにgoose固有のコメント形式を覚え、ツールの版を管理する必要がある。Downは削除したデータの復元を保証せず、特に初期テーブルの削除は認証データを失うため通常操作にはしない。接続設定の安全な渡し方、権限、transaction・並行実行制御、失敗後の復旧、版固定は導入前に検証する。採用は文書上の合意であり、ツールの導入・migration適用は未実施。

一次情報：[gooseのSQL形式](https://pressly.github.io/goose/documentation/annotations/)、[CLI](https://pressly.github.io/goose/documentation/cli-commands/)、[sqlcのmigration対応](https://docs.sqlc.dev/en/stable/howto/ddl.html)、[golang-migrate](https://github.com/golang-migrate/migrate)。


### Node.jsの開発用バージョンとnpm依存の管理

利用者からNodeの脆弱性を懸念する意向があり、開発用Nodeは26.13.0 LTSをasdfで導入し、ルートの `.tool-versions` で固定する。Viteを採用する方針は維持する。npm依存の具体的な構成と以下の運用方針は、依存導入時に確認する提案として区別する。

- Vue・Vite・PrimeVue等の依存を導入する際に、採用する版とNode 26.13.0の互換性を確認する。旧AI案のNode 22固定条件を自動採用しない。
- バージョン固定は更新を止める目的ではない。Node公式のセキュリティ情報とサポート期間を確認し、修正版が出たら影響確認・ビルド・テストを行って更新する。特定の版に脆弱性がないことを保証しない。
- Node本体とnpm依存を分けて確認する。npm依存はlockfileと `npm ci` で再現性を確保し、開発用を含めた `npm audit` と依存の変更内容を確認する。auditはNode本体の修正状況や未知の問題・悪意あるパッケージがないことを証明するものではない。自動の強制更新を安全性確認の代替にしない。
- Node/Viteは開発・ビルドに使う。本番はViteで作った静的ファイルをnginxで配信し、APIはGoで動かす既存方針を維持する。NodeのHTTPサーバーやVite開発サーバーを本番公開しない。開発・依存取得・ビルド中のリスクは残るので、不要な依存を増やさず、本番の秘密をFrontendへ渡さない。

一次情報：[Nodeのサポート状況](https://nodejs.org/en/about/previous-releases)、[Node 26.13.0 LTS](https://nodejs.org/en/blog/release/v26.13.0)、[2026年10月のセキュリティ修正](https://nodejs.org/en/blog/vulnerability/october-2026-security-releases)、[npm audit](https://docs.npmjs.com/cli/v11/commands/npm-audit/)、[Viteの本番ビルド](https://vite.dev/guide/build.html)。採用依存の監査・互換性試験は依存導入時に行う。

## ブラウザ側のデータ取得

ブラウザ側のcacheは積極的に永続化しない。TanStack Queryなどを採用する場合も、次をWatchBriefの画面仕様に合わせて明示設定する。

- GETの自動retryと回数
- window focus時の再取得
- inactive queryのメモリ保持時間
- HTTP cacheとの責務分担
- 画面に表示する再試行操作との整合

CRUDなど状態を変更する処理は、重複変更を避けるため無条件の自動retryを行わない方向で検討する。具体的なHTTPエラー分類とretry条件はIssue #7で決める。

## 技術選定の進め方

1. 認証、生成ジョブ、インフラなど、選定に必要な要件を対象Issueで整理する。
2. 候補ごとの学習コスト、実装量、運用性、障害時の調査方法を比較する。
3. 必要なら小さな縦切り検証を行い、推測ではなく結果を記録する。
4. 採用理由、見送った案、影響範囲をIssueへ記録する。
5. 決定を `docs/system-design.md`、セットアップ手順、依存関係定義へ反映する。

## 関連Issue

- [#1 認証・公開境界・秘密情報](https://github.com/smofktn/watchbrief/issues/1)
- [#7 生成ジョブの失敗時動作・ログ・再実行](https://github.com/smofktn/watchbrief/issues/7)
- [#9 WatchBriefのアプリ構想](https://github.com/smofktn/watchbrief/issues/9)
- [#11 EC2 1台構成のインフラ・運用](https://github.com/smofktn/watchbrief/issues/11)
