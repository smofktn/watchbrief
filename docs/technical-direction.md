# WatchBrief 技術選定の前提

> [!NOTE]
> この文書は、技術を選ぶときの目的、重視する性質、現在の候補を整理する。候補は採用決定ではない。決定した内容は、理由と影響を記録した上でシステム設計や実装文書へ反映する。

## プロジェクトの目的

WatchBriefの主目的は、個人向けの情報整理Webアプリを作ることである。補助目的として、AWSを使ったサーバー構築と運用を学ぶ。

学習目的であっても、最初から構成を過度に複雑にしない。実際のアプリに意味があり、情報や利用事例が多く、今後にも応用しやすい技術を優先する。

## 技術選定で重視すること

- 初心者でも構築、障害調査、復旧の流れを理解できること
- 小さく開始し、必要になった段階で拡張できること
- エラーハンドリングと入力検証を明示的に実装できること
- 非同期処理、timeout、部分失敗、重複実行、復旧を扱えること
- DB設計とドメイン設計を実装都合だけで崩さないこと
- 構造化ログと監視により、画面、API、Job、Attemptを追跡できること
- 性能を推測だけで最適化せず、代表的な処理を計測できること
- 認証情報や利用者データをPublicリポジトリへ含めないこと

過度なCQRSやイベントソーシングは初期候補にせず、必要十分な責務分離を目指す。軽量なDDDとPorts and Adaptersは候補だが、frameworkやディレクトリ構成を決める前に固定しない。

## 現在の候補

| 領域 | 現在の候補・意向 | 状態 |
|---|---|---|
| リポジトリ | Web、worker、共通ドメイン、インフラを同じリポジトリで扱うmonorepo | 有力候補。workspace構成は未決定 |
| 初期インフラ | AWSのEC2 1台でWeb、worker、schedulerを運用 | 第一候補。Issue #11で設計する |
| 将来のインフラ | ECS/Fargate | 運用負荷や拡張要件が生じた場合の移行候補 |
| Frontend | Vue + TypeScript + Vite + Vue Router | 経験を活かせる第一候補。Reactとも比較する |
| Frontend状態管理 | TanStack Query | サーバー状態管理の候補。自動retry・再取得・cache既定値はそのまま採用しない |
| Client store | Piniaは初期候補にしない | 必要性が生じた場合に再検討する |
| Backend | Go | 第一候補。経験のあるJava/Spring Bootも開発速度の比較対象にする |
| DBアクセス | GORM、または `database/sql` + sqlc | 比較候補。ドメインモデルをORM構造体へ過度に依存させない |
| migration | versioned migration | 優先候補。本番起動時の自動migrationだけに依存しない |
| DB | SQLite、PostgreSQL、RDSを含む構成 | 未決定 |
| AI | Amazon Bedrock、OpenAI API、または併用 | 未決定。BedrockはAWS学習との相性から候補 |
| 情報収集 | AIサービスとは別のWeb検索・取得手段 | 未決定 |
| IaC | ツールと導入時期を含め未決定 | Issue #11で検討する |

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
