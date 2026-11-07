# WatchBrief DB設計

> **Issue #53の既存案について**：以前のAIが作成した未完成の実装案と、それに伴う文書変更を参考として保持している。具体バージョン、ハッシュ、Cookie・CSRF、migration等は、今回利用者と確認したもの以外は採用済みと扱わない。過去の検証結果も今回の実動作確認とは区別する。現在の進め方は[作業計画](work-plan.md#利用者主体で子issueを進める運用合意済み)を参照する。

> [!NOTE]
> この文書はIssue #3で整理した論理スキーマ案を示す。初期DB製品はIssue #11でPostgreSQLに決定した。認証のUser・AuthSessionは[Issue #53の物理スキーマ](#issue-53の認証物理スキーマ)として具体化し、それ以外の英語名・物理型は引き続き実装候補とする。

## 目的とドメインモデルとの関係

アプリの概念と業務ルールを、DBのテーブル、カラム、外部キー、制約、インデックスとしてどう保存・保護するかを示す。業務上の意味と状態遷移は[ドメインモデル](domain-model.md)、利用者向け仕様は[プロダクト仕様](product-definition.md)を参照する。

アプリのクラス・構造体とテーブルを1対1にすることは要求しない。例えば、事実が持つ出典の一覧は、DBでは `REPORT_SOURCES` と中間テーブル `FACT_SOURCES` で表す。ORMへのマッピングとアプリ側の型構成は実装時に決める。

## スキーマを確定する段階

- Issue #3：保存対象、テーブル案、関係、論理カラム、一意性、削除方針、初期インデックスの要件を整理する。
- 物理設計時：PostgreSQLと採用するDBアクセス方式に合わせ、物理型、文字数上限、NULL・CHECK制約、インデックス、ロック方式を具体化する。
- 実装時：migrationとしてレビュー・適用できる形にし、制約、トランザクション、主要クエリを検証する。認証部分は[Issue #53の物理スキーマ](#issue-53の認証物理スキーマ)と番号付きSQLへ具体化した。

## 保存設計の前提と制約

- 初期DBはPostgreSQLとし、アプリと同じEC2に配置する。将来RDSへ分離する方針にIssue #11で合意し、初期からRDSを使う以前の決定は更新した。認証のローカル対象版・物理型・DBアクセス・migrationは[Issue #53](#issue-53の認証物理スキーマ)で具体化した。将来の移行先エンジン（Auroraを含む比較）、サイズ、可用性構成と他テーブルの物理型等は未決定であり、論理スキーマは維持する。
- 初期PostgreSQLの正本データは、OS用とは別のDB用EBSに保存し、EC2削除時も保持する（`DeleteOnTermination=false`）。OS用・DB用とも東京リージョンのAWS管理KMS鍵 `aws/ebs` で暗号化する。容量、種類、ファイルシステム、保存場所、権限、マウント、保持後の管理・削除手順は未決定である。詳細は[システム設計](system-design.md#初期ストレージの方針)で扱う。
- 初期PostgreSQLはEC2内部からの接続に限定し、待受設定とSecurity Groupで外部からの直接接続を許可しない。具体的なアドレス・ポート・ソケット方式、DB認証、保存方法、DB管理操作の具体手順は今後設計する。管理接続は[システム設計](system-design.md#初期sshの接続元と認証)を参照する。将来RDSを使う場合も非公開とし、通常のDB接続元をアプリ実行環境に限定する。
- 予定された生成を `GenerationJob`、workerによる実際の1回の実行を `GenerationAttempt` として分ける。
- 待機中のJobにはAttemptが存在しない。実行を開始した場合は、成功・失敗にかかわらずAttemptを残す。
- 実行開始時はGenerationAttemptの作成とGenerationJobの `running` への変更を同じDBトランザクションで行い、片方だけを残さない。
- GenerationJobの状態は `queued`、`running`、`succeeded`、`failed` とする。
- GenerationAttemptの状態は `running`、`succeeded`、`failed` とし、待機状態は持たせない。
- Job・Attemptの画面表示は[プロダクト仕様](product-definition.md#レポート)を参照する。DBでは上記の内部状態を保存する。
- timeoutは独立した状態にせず、GenerationJobとGenerationAttemptを `failed` にした上で、GenerationAttemptの `failure_type` に `timeout` を記録する。
- 実行中のworkerはGenerationAttemptの `heartbeat_at` を定期的に更新する。別の監視処理が一定時間更新されていない `running` Attemptを検出した場合、JobとAttemptを同じDBトランザクションで `failed` にし、`failure_type` を `worker_interrupted`、`finished_at` を検出時刻とする。具体的な時間設定はIssue #7で決める。
- 停止判定後に遅れて元のworkerが戻った場合、Attemptが `running` であることを条件に状態を更新し、失敗確定済みのAttemptを成功へ戻したりReportを保存したりしない。
- GenerationAttemptからReportへの関係は0または1件とする。`succeeded` のAttemptには必ず1件、`failed` のAttemptには0件とし、DBでReportの `generation_attempt_id` を一意にして同じAttemptから複数のReportが保存されることを防ぐ。
- 成功時はReportと配下の構造化データ、GenerationAttemptの `succeeded`・`finished_at`、GenerationJobの `succeeded` を同じDBトランザクションで保存する。
- Report保存を含む成功トランザクションが失敗した場合は全体をロールバックし、別トランザクションでGenerationAttemptとGenerationJobを `failed`、`failure_type` を `persistence_error` として記録する。Report保存前に確定した既知の失敗も、AttemptとJobを同じトランザクションで失敗へ変更する。
- DB障害により失敗状態の記録もできない場合は、元の失敗と記録失敗をDB外のプロセスログへ残す。`running` のまま残ったAttemptはheartbeat監視で後から検出する。
- AIは完成MarkdownではなくMarkdown断片を含む構造化データを出力し、アプリが固定構成のMarkdown本文を組み立てる。組み立てた本文は `Report` の一部として構造化データと同じトランザクションでDBへ保存し、閲覧時に再生成しない。S3は本文の主保存先にはしない。
- Markdownの許可構文・表示処理は[プロダクト仕様](product-definition.md)を参照する。DBは検証済み構造化データと完成本文を保存する。
- 初期MVPでは自動retryを行わないため、1つのJobに対するAttemptは最大1件とする。将来retryを導入する場合は、同じJobにAttemptを追加できる関係を使う。`attempt_number` はJobごとに1から始め、DBで `generation_job_id` と `attempt_number` の組み合わせを一意にする。
- 初期MVPではGenerationJob、GenerationAttempt、Reportを削除しない。GenerationAttemptは必ず1つのGenerationJobを、Reportは必ず1つのGenerationAttemptを参照し、子がある親の削除は外部キーの `ON DELETE RESTRICT` で拒否する。Attemptがない `queued` Jobにも削除操作を用意しない。
- SchedulerはJob作成時点の生成入力を、GenerationJobの `input_snapshot` JSONカラムへ保存する。Jobとスナップショットを同じDBトランザクションで保存し、保存後は変更しない。workerは現在設定ではなくスナップショットを使う。
- `input_snapshot` には `schema_version`、調査方針、初回の更新確認期間の決め方、有効なWatchTargetのID・名前・補足指示・表示順、InformationSourceのID・名前・URL・補足指示を含める。日数指定の場合は指定日数も含める。JSON内のIDは履歴用の値であり、現在設定への外部キーにしない。cron式、認証情報、token、secretは含めず、Collection名はJobの `collection_name` を使う。
- AI出力は保存前に検証し、未検証の生レスポンスを保存しない。対象・出典との対応や必須内容の検証規則は[システム設計の生成フロー](system-design.md#レポート生成フロー)と[プロダクト仕様](product-definition.md)を参照する。
- CollectionをOFFにした後は、`queued` または `running` のJobがあってもWatchTarget、InformationSource、調査方針、CollectionScheduleを編集・削除できる。作成済みJobは `input_snapshot` を使い、現在設定の変更は後続Jobだけに反映する。Collection本体は、既決定どおり稼働中Jobがある間は削除できない。
- `queued` または `running` のGenerationJobがあるCollectionは削除できない。すべてのJobが `succeeded` または `failed` になった後はCollectionを物理削除できるが、GenerationJob、GenerationAttempt、Reportは保持する。GenerationJobの `collection_id` はNULLを許可し、Collection削除時は外部キーの `ON DELETE SET NULL` で参照だけを外す。
- 現在設定の親子関係にはDBの `ON DELETE CASCADE` を使う。Collection削除時はCollectionScheduleとWatchTargetを、WatchTarget削除時はInformationSourceを連鎖削除する。生成履歴であるGenerationJob、GenerationAttempt、ReportとReport配下の記録には、この連鎖削除を適用しない。
- 初期MVPではUserを削除しない。UserからCollectionとGenerationJobへ `ON DELETE CASCADE` を設定せず、所有データがあるUserの削除は外部キーの `ON DELETE RESTRICT` で拒否する。Reportの所有UserはGenerationJobから判定する。将来User削除が必要になった場合は、削除範囲、確認、バックアップを改めて設計する。
- Collection削除とSchedulerによるJob作成は、同じCollectionを基準にDBトランザクションで排他制御する。削除が先に確定した場合はJobを作らず、Job作成が先に確定した場合はCollection削除を拒否する。
- Collectionだけが編集可能な名前の正本を持つ。GenerationJobにはJob作成時のコレクション名を履歴用にコピーし、GenerationAttemptとReportはコレクション名を重複して持たない。
- Reportは `user_id` を持たず、GenerationAttemptとGenerationJobを介して所有Userを判定する。同じ所有者情報の重複と、Jobとの不整合を避けるためである。
- GenerationJobの `user_id` は必須とする。Collection削除により `collection_id` がNULLになった後も、Job、Attempt、Reportの所有Userを判定できるようにするためである。
- 同じCollectionと同じ予定時刻に対応するGenerationJobは1件だけとする。DBで `collection_id` と `scheduled_at` の組み合わせを一意にし、重複作成の競合は失敗ではなく作成済みとして扱う。専用の冪等性キーカラムは持たない。
- Scheduleの編集・削除は将来作成するGenerationJobだけに影響する。作成済みJobの `scheduled_at` は変更せず、過去一覧は保存済みJobを正本として表示する。現在のcron式から過去予定を再計算せず、Jobが作られなかった予定も復元しない。
- Schedulerはcron式を日本時間として解釈し、該当した予定を `scheduled_at` としてJobへ保存する。日時はDBへUTCで保存し、画面では日本時間に変換して表示する。
- SchedulerがGenerationJobをDBへ保存する前に失敗した場合、初期MVPでは存在しないJobを予定情報から復元しない。画面上の「待機中」は、DBに存在する `queued` のJobだけを表し、Job作成失敗はSchedulerのログと監視で検知する。
- CollectionScheduleは1つのCollectionにつき最大1件とし、DBで `collection_id` を一意にして重複登録を防ぐ。1日に複数回生成する場合も、複数のScheduleではなく1つのcron式で表す。
- InformationSourceはWatchTargetごとに0件以上とする。InformationSourceがなくてもWatchTargetを保存でき、最後の1件も削除できる。登録がない場合は、対象名とAIへの補足指示を使って公開情報を調査する。
- 初期MVPでは、同じWatchTarget内のInformationSource URLについて重複検査や一意制約を設けない。URLの重複防止と正規化は将来の改善候補とする。
- CollectionとWatchTargetの `display_order` には一意制約を設けない。並べ替えでは関連する行を1つのトランザクションで更新し、取得時は `display_order`、同値の場合はIDの順に並べて表示を安定させる。
- すべてのエンティティのIDをULIDに統一し、UUIDや整数の自動採番とは使い分けない。認証部分のPostgreSQL上の物理型、生成方法、検証方法は[Issue #53](#issue-53の認証物理スキーマ)を参照し、他テーブルは実装時に具体化する。
- Userの `login_id` はアプリ全体で一意とし、DBの一意制約で重複を防ぐ。作成時とログイン時に英字を小文字へ正規化し、DBには英小文字 `a-z`、数字 `0-9`、`_`、`-` だけを1文字以上64文字以下で保存する。
- User、Collection、CollectionSchedule、WatchTarget、InformationSourceには、汎用的な作成日時と更新日時を持たせる。
- Collectionは、初回レポートの更新確認開始をAIに任せるか日数で指定するかを持ち、初期値はAIに任せる方式とする。日数指定の場合だけ1以上365以下の遡る日数を持つ。AIも初回開始日時を1日以上365日以下の範囲で選ぶ。現在の状態は、この期間とは別に毎回確認する。
- GenerationJob、GenerationAttempt、ReportとReport配下の記録には、一律の作成日時・更新日時を持たせず、その記録で意味のある日時だけを持たせる。GenerationJobは予定を表す `scheduled_at` と実際のキュー登録を表す `queued_at`、GenerationAttemptは実行の `started_at`、`heartbeat_at`、`finished_at`、Reportは更新・新着情報の確認開始を表す `updates_started_at` を持つ。
- 成功したGenerationAttemptの `started_at` を更新確認期間の終了、`finished_at` をReportの生成完了日時として使う。Reportには同じ値になる期間終了日時と生成日時を重複して持たせない。
- 初期MVPでは、作成者IDと更新者IDをどのテーブルにも持たせない。将来、変更者の追跡が必要になった場合は、独立した監査記録として検討する。

## 通常実行と管理操作のDB権限（Issue #1での方針合意）

通常のアプリ実行用と、テーブル構造変更・DBユーザー管理などの管理用の権限を分ける。通常のアプリには必要なデータ操作だけを許可し、DDLとDBユーザー管理の権限を与えない。これは閲覧・変更する行の所有者をAPIが検証する認可とは別の制御であり、両方が必要である。具体的なDBロール名、API・Scheduler・Worker・停止監視ごとの権限配分、migration・初期ユーザー作成・パスワード再設定用コマンドの接続権限は各実装・構築時に設計・検証する。現在のローカル再設定権限は[再設定の詳細](password-reset.md#最小db権限と索引)を参照する。権限分離の方針自体はテーブル・カラム・業務上の削除方針を変更しない。

## 認証セッション（Issue #1での方式合意）

初期認証はPostgreSQLに保存するサーバー側セッションを使う。論理テーブル `AUTH_SESSIONS` はUserに属し、Cookieで受け取ったランダムトークンを照合してUserを特定する。生トークン、パスワード、個人設定、Report本文をこのテーブルへ保存しない。

| 論理カラム | 制約・役割 |
|---|---|
| `id` | 主キー。既存のエンティティID方針に従うULID。認証トークンとは別物 |
| `user_id` | 必須。Userへの外部キー。User削除は初期MVPで提供せず、参照中の削除はRESTRICTで拒否 |
| `token_hash` | 必須・一意。crypto/randで作る32byte乱数のSHA-256。生トークンは保存せず、Cookieの値から計算して照合する |

Userは0件以上の認証セッションを持ち、各セッションは必ず1 Userに属する。APIは `token_hash` の一意検索を使う。認証成功時に保存し、ログアウト後は同じトークンで認証できないよう無効化する。トークン生成・ハッシュとCookie更新は[現在の基本設計](system-design.md#issue-53の基本設計)の合意に従う。#54のログアウトは[対象行の物理削除](#issue-54のログアウト保存処理)に合意した。再設定の物理DELETE・競合制御は[管理コマンド](password-reset.md)に従う。不要レコード整理等は後続の担当Issueで具体化する。ログイン側のCookie表現はパディングなしBase64URL、DBは元32byteのSHA-256（bytea・32byte）とする。時間による失効や複数端末ログアウト管理を今回追加するものではない。

管理コマンドによるパスワード再設定時は例外として、Userのパスワードハッシュ更新と、そのUserに属する既存セッションすべての無効化を同じDBトランザクションで確定する。途中で失敗した場合は全体をロールバックし、片方だけを残さない。他Userのセッション、Collection設定、Job・Attempt・Reportは更新しない。対象user_idの全sessionをDELETEし、auth_sessions(user_id)の索引を使う。同時ログイン・同時再設定との競合でも旧パスワードで認証したセッションが無効化をすり抜けないよう、[共通transaction advisory lockとPHC再読取](password-reset.md#ログインとの競合制御)で排他制御する。再設定方針を表す追加カラムはここで固定せず、無効化方式に応じて物理設計で選ぶ。

## ER図

各ボックスの見出しは `英語名 / 日本語名`、各カラムの末尾は日本語での意味を表す。英語名は実装時の候補であり、物理テーブル名・カラム名を確定するものではない。

```mermaid
erDiagram
    USERS ||--o{ AUTH_SESSIONS : 認証状態を持つ
    AUTH_SESSIONS["AUTH_SESSIONS / 認証セッション"] {
        identifier id PK "セッション記録ID（ULID）"
        identifier user_id FK "認証済みユーザーID"
        string token_hash UK "ランダムトークンの照合用ハッシュ"
    }
    USERS ||--o{ COLLECTIONS : 所有する
    COLLECTIONS ||--o| COLLECTION_SCHEDULES : 設定する
    COLLECTIONS ||--o{ WATCH_TARGETS : 含む
    WATCH_TARGETS ||--o{ INFORMATION_SOURCES : 持つ

    USERS ||--o{ GENERATION_JOBS : 所有する
    COLLECTIONS o|--o{ GENERATION_JOBS : 生成元になる
    GENERATION_JOBS ||--o{ GENERATION_ATTEMPTS : 実行される
    GENERATION_ATTEMPTS ||--o| REPORTS : 生成する

    REPORTS ||--o{ REPORT_TARGET_RESULTS : 含む
    REPORT_TARGET_RESULTS ||--o{ REPORT_FACTS : 事実を持つ
    REPORT_TARGET_RESULTS ||--o{ REPORT_SOURCES : 参照する
    REPORT_FACTS ||--o{ FACT_SOURCES : 出典を持つ
    REPORT_SOURCES ||--o{ FACT_SOURCES : 引用される
    REPORT_TARGET_RESULTS ||--o{ REPORT_SOURCE_FAILURES : 取得失敗を記録する

    USERS["USERS / ユーザー"] {
        identifier id PK "ユーザーID"
        string login_id UK "ログインID（一意）"
        string password_hash "パスワードハッシュ"
        datetime created_at "作成日時"
        datetime updated_at "更新日時"
    }

    COLLECTIONS["COLLECTIONS / コレクション"] {
        identifier id PK "コレクションID"
        identifier user_id FK "所有ユーザーID"
        string name "コレクション名"
        text research_instruction "調査方針"
        boolean enabled "有効状態"
        string initial_lookback_mode "初回の更新確認期間の決め方（初期値AI）"
        int initial_lookback_days "初回に遡る日数（日数指定時のみ、1〜365）"
        int display_order "表示順"
        datetime created_at "作成日時"
        datetime updated_at "更新日時"
    }

    COLLECTION_SCHEDULES["COLLECTION_SCHEDULES / 生成スケジュール"] {
        identifier id PK "生成スケジュールID"
        identifier collection_id FK, UK "コレクションID（一意）"
        string cron_expression "cron式"
        datetime created_at "作成日時"
        datetime updated_at "更新日時"
    }

    WATCH_TARGETS["WATCH_TARGETS / ウォッチ対象"] {
        identifier id PK "ウォッチ対象ID"
        identifier collection_id FK "コレクションID"
        string name "ウォッチ対象名"
        text instruction "AIへの補足指示"
        boolean enabled "有効状態"
        int display_order "表示順"
        datetime created_at "作成日時"
        datetime updated_at "更新日時"
    }

    INFORMATION_SOURCES["INFORMATION_SOURCES / 情報源"] {
        identifier id PK "情報源ID"
        identifier watch_target_id FK "ウォッチ対象ID"
        string name "情報源名"
        string url "情報源URL"
        text instruction "AIへの補足指示"
        datetime created_at "作成日時"
        datetime updated_at "更新日時"
    }

    GENERATION_JOBS["GENERATION_JOBS / 生成ジョブ"] {
        identifier id PK "生成ジョブID"
        identifier user_id FK "所有ユーザーID"
        identifier collection_id FK "生成元コレクションID（削除後NULL）"
        string collection_name "Job作成時のコレクション名"
        json input_snapshot "Job作成時の生成入力"
        datetime scheduled_at "実行予定日時"
        datetime queued_at "キュー登録日時"
        string status "ジョブ状態"
    }

    GENERATION_ATTEMPTS["GENERATION_ATTEMPTS / 生成試行"] {
        identifier id PK "生成試行ID"
        identifier generation_job_id FK "生成ジョブID"
        int attempt_number "試行番号"
        datetime started_at "実行開始日時"
        datetime heartbeat_at "workerの最終生存確認日時"
        datetime finished_at "実行終了日時"
        string status "試行状態"
        string failure_type "失敗種別"
    }

    REPORTS["REPORTS / レポート"] {
        identifier id PK "レポートID"
        identifier generation_attempt_id FK, UK "生成試行ID（一意）"
        text body_markdown "生成済みMarkdown本文"
        text summary_markdown "今回の変化のMarkdown断片"
        datetime updates_started_at "更新・新着情報の確認開始日時"
    }

    REPORT_TARGET_RESULTS["REPORT_TARGET_RESULTS / 対象別調査結果"] {
        identifier id PK "対象別調査結果ID"
        identifier report_id FK "レポートID"
        identifier target_id "生成時のウォッチ対象ID（履歴値）"
        string target_name "生成時のウォッチ対象名"
        int display_order "表示順"
        string status "調査状態"
        text ai_summary_markdown "AIによる整理のMarkdown断片"
    }

    REPORT_FACTS["REPORT_FACTS / 確認済み事実"] {
        identifier id PK "確認済み事実ID"
        identifier report_target_result_id FK "対象別調査結果ID"
        string fact_kind "現在状態または更新・新着の区分"
        text content_markdown "事実のMarkdown断片"
        int display_order "表示順"
    }

    REPORT_SOURCES["REPORT_SOURCES / レポート情報源"] {
        identifier id PK "レポート情報源ID"
        identifier report_target_result_id FK "対象別調査結果ID"
        identifier source_id "生成時の登録情報源ID（履歴値、発見時はNULL）"
        string source_kind "情報源の由来（registered/discovered）"
        string source_name "情報源名"
        string source_url "出典URL"
    }

    FACT_SOURCES["FACT_SOURCES / 事実の出典"] {
        identifier id PK "事実の出典ID"
        identifier report_fact_id FK "確認済み事実ID"
        identifier report_source_id FK "レポート情報源ID"
    }

    REPORT_SOURCE_FAILURES["REPORT_SOURCE_FAILURES / 情報源取得失敗"] {
        identifier id PK "情報源取得失敗ID"
        identifier report_target_result_id FK "対象別調査結果ID"
        identifier source_id "生成時の登録情報源ID（履歴値、発見時はNULL）"
        string source_kind "情報源の由来（registered/discovered）"
        string source_name "情報源名"
        string source_url "情報源URL"
        string reason "失敗理由"
    }
```

`identifier` はULIDを表す。図では論理的な型だけを示し、PostgreSQL上の物理型はORMなどの選定と合わせて決める。

## 読み方

- `USERS` はログイン可能な利用者。ユーザー作成とパスワード再設定は、画面ではなく管理コマンドで扱う。
- `AUTH_SESSIONS` はUserに属する認証状態を表す。Cookieのトークン照合と失効の要件は[認証セッション](#認証セッションissue-1での方式合意)を参照する。
- `COLLECTIONS` はユーザーごとの調査単位。調査方針、停止状態、表示順、初回の更新確認期間の決め方を持つ。`initial_lookback_mode` はAIに任せる方式と日数指定方式を区別し、初期値はAIに任せる方式とする。日数指定の場合だけ、1以上365以下の `initial_lookback_days` を使う。
- `WATCH_TARGETS` はコレクションに属する。対象をアプリ全体で共通化せず、別コレクションでは別の設定として登録する。対象名を必須、AIへの補足指示を任意とする。
- `INFORMATION_SOURCES` はウォッチ対象に0件以上紐付く。登録する場合は名称とURLを必須、AIへの補足指示を任意とする。初期MVPでは同じウォッチ対象内のURL重複を許可する。
- `COLLECTION_SCHEDULES` は日本時間のcron式を表す。1コレクションにつき最大1件とし、`collection_id` の一意制約で保証する。
- `GENERATION_JOBS` は予定された生成を表す。ユーザーに属し、Job作成時のコレクション名と生成入力のJSONスナップショットを保持する。待機中はAttemptを持たず、workerが処理を開始した時点でAttemptを持つ。`collection_id` は現在存在するCollectionへの任意の参照であり、Collection削除後は `ON DELETE SET NULL` によりNULLとなる。生成履歴の起点として、初期MVPでは削除しない。
- `GENERATION_ATTEMPTS` はworkerによる実際の1回の実行を表し、必ず1つのGenerationJobを参照する。実行開始・最終heartbeat・終了時刻と成功・失敗などの結果を記録する。初期MVPでは1つのJobにつき最大1件だが、将来retryする場合は複数件を持てる。
- `REPORTS` は成功したGenerationAttemptにだけ紐付く生成済みレポート。所有Userとコレクション名はGenerationAttemptを介してGenerationJobから取得する。AIの構造化出力からアプリが組み立てたMarkdown本文と、今回の変化を表すMarkdown断片をDB内に保持し、閲覧時は保存済みの本文を表示して、AIや情報源を再実行しない。
- `REPORT_TARGET_RESULTS` はレポート内の対象ごとの結果。生成時の `target_id` を履歴値として持つが、現在のウォッチ対象設定への外部キーにはしない。同じReport内で同じ `target_id` を重複させない。
- `REPORT_FACTS` は、現在状態または更新・新着の事実のMarkdown断片を表す。
- `REPORT_SOURCES` は対象ごとに利用できた情報源を保持する。Collection設定からコピーした `registered` と、AIが生成中に見つけた `discovered` を区別する。`registered` は生成時の `source_id` を履歴値として持ち、`discovered` の `source_id` はNULLとする。どちらも現在設定への外部キーにはしない。
- `FACT_SOURCES` は事実と `REPORT_SOURCES` の対応を表す。同じ情報源を複数の事実から参照しても、名称とURLを重複保存しない。事実と情報源は同じ `REPORT_TARGET_RESULTS` 配下であることをアプリで検証し、同じ事実と情報源の組み合わせを重複させない。
- `REPORT_SOURCE_FAILURES` は一部情報源の取得失敗を表し、こちらも `registered` と `discovered` を区別する。情報源の一部が失敗しても、レポート全体は保存できる。

`queued` または `running` のJobがある間はコレクションを削除できない。すべてのJobが `succeeded` または `failed` になった後にコレクションを削除すると、`ON DELETE CASCADE` によりコレクション、ウォッチ対象、情報源、cronスケジュールを物理削除する。完了済みのJob、Attempt、Reportは削除せず保持する。

## 保存とバックアップ

レポートのMarkdown本文は、構造化データと同じDBを正本とする。初期MVPではDBバックアップと外部アーカイブを実装しない。将来、AWS側のDBバックアップをDB全体の正式な復旧手段、非公開GitリポジトリへのMarkdown保存を本文の副次アーカイブとして追加する。Gitへの保存はDB保存後の独立した処理とし、失敗しても保存済みReportの成功状態を変更しない。

初期のDB用EBS分離・EC2削除時の保持は、正本データをOS・アプリと別ディスクに置いてEC2と連動して削除しないためのものであり、バックアップではない。DB破損や誤削除からの復元は保証しない。

将来RDSへ移す際は、自動バックアップの可否・保持期間などを具体構成の選定時に確認する。初期MVP対象外という合意は、将来利用するRDSの標準機能を無効化する指示ではない。初期EC2構成も将来の移行も、今回の合意だけではバックアップ設定や復元手順・試験を確定せず、データ復旧を保証しない。失って困る実データを継続運用する前に方針を再確認する。

## 初期インデックス方針

初期MVPでは、次の検索経路を支えるインデックスを用意する。主キーと一意制約によって作られるインデックスは重複して作らない。実際のインデックス定義はPostgreSQLと採用するDBアクセス方式に合わせて確認する。

- 一意制約: `auth_sessions(token_hash)`、`users(login_id)`、`collection_schedules(collection_id)`、`generation_jobs(collection_id, scheduled_at)`、`generation_attempts(generation_job_id, attempt_number)`、`reports(generation_attempt_id)`、`report_target_results(report_id, target_id)`、`fact_sources(report_fact_id, report_source_id)`。
- 設定一覧: `collections(user_id, display_order, id)`、`watch_targets(collection_id, display_order, id)`、`information_sources(watch_target_id, id)`。
- Jobの一覧と処理: `generation_jobs(user_id, scheduled_at, id)`、`generation_jobs(status, scheduled_at, id)`、`generation_jobs(collection_id, status)`。
- レポート配下の取得: `report_target_results(report_id, display_order, id)`、`report_facts(report_target_result_id, display_order, id)`、`report_sources(report_target_result_id, id)`、`report_source_failures(report_target_result_id, id)`。

これにより、ユーザーの日付別履歴、コレクション別履歴、workerによる待機Job取得、Collection削除前の稼働中Job確認、外部キーの参照確認、表示順に沿った子要素取得を支える。初期MVPでは、Markdown本文の全文検索、`input_snapshot` JSON内の検索、情報源URL検索のインデックスは作らない。実装後に代表的な一覧・取得クエリの実行計画を確認し、DBやORMが自動作成するものとの重複を除き、不足が確認された場合だけ追加する。

## 物理設計・実装で具体化すること

- PostgreSQLのバージョン、ORM、物理テーブル名・カラム名、データ型、文字数上限。
- ULIDの保存・生成・検証方法、日時とJSONの物理型。
- NULL・CHECK制約を含む具体的なDDLとmigration方式。
- Collection削除とJob作成、workerと停止監視の競合に使う具体的なロック・条件付き更新方式。
- 主要クエリと実行計画に基づくインデックスの調整。

Markdownライブラリ、timeout・heartbeat間隔、queue・監視などの実行方式は[システム設計](system-design.md)とIssue #7で具体化する。

## Issue #53の認証物理設計（保存構成と型に合意）

対象をUserと認証セッションの2テーブルに絞り、文字列をtext、トークンハッシュをbyteaで保存する構成に合意した。ID・トークンハッシュの重複拒否、Userへの外部キー、ULIDの検証をDBでも行う。次節の旧AI案を採用する決定ではない。下表は合意した保存構成・型を具体化したもので、SQL上の細かな制約・既定値・索引・符号化形式は実装時に確認する。migrationには[gooseによる番号付きSQL](technical-direction.md#migrationツールgoose採用)を使う。ドライバー・ツールの具体版は別途決める。

| 保存先 | PostgreSQL型 | 制約・役割（細部は実装時に確認） |
|---|---|---|
| `users.id` | `text` | 主キー。26文字の正規ULIDを検証 |
| `users.login_id` | `text` | NOT NULL・UNIQUE。小文字の `a-z`・数字・`_`・`-`、1〜64文字をCHECK。正規化はアプリで行う |
| `users.password_hash` | `text` | NOT NULL。方式・パラメーター・saltを含むハッシュ文字列を保存。具体的な符号化・上限は実装時に確認 |
| `users.created_at` / `updated_at` | `timestamptz` | NOT NULL。Userの作成・更新日時。日時を持つ方針は既存合意。具体型はtimestamptz、既定値はCURRENT_TIMESTAMPを使用 |
| `auth_sessions.id` | `text` | 主キー。認証トークンとは別のULIDを検証 |
| `auth_sessions.user_id` | `text` | NOT NULL、Userへの外部キー。参照中のUser削除はRESTRICT |
| `auth_sessions.token_hash` | `bytea` | NOT NULL・UNIQUE、32byteをCHECK。元トークンのSHA-256の結果を保存 |

文字列はtext、SHA-256の生byte列はbyteaを使う。ULIDをchar(26)で扱う場合の空白paddingや、トークンハッシュを文字列化してからDB保存する変換を避ける。長さ・文字種は型だけに任せずCHECKで検証し、主キー・一意制約の索引を重複作成しない。追加の索引・照合順序・作成時の既定値等は実装時に確認する。独自DB拡張はこの案に含めない。

セッションに期限・最終アクセス日時は追加しない。セッション作成日時の要否は未決定とし、旧AI案にあることだけで追加しない。1 Userに複数セッションを持てる既存関係を維持する。User作成コマンドとAPIの権限分離、DBからの所有者昇格拒否等はDDLの設計と合わせて検証する。

一次情報：[PostgreSQL文字列型](https://www.postgresql.org/docs/current/datatype-character.html)、[バイナリー型](https://www.postgresql.org/docs/current/datatype-binary.html)、[制約](https://www.postgresql.org/docs/current/ddl-constraints.html)。保存構成・型の合意と現在のDDLは次のとおり。実行した検証結果と未確認の範囲はIssueへ記録する。

現在のDDLは [goose migration](../backend/db/goose/000001_create_auth.sql) で管理する。ID・login_idはASCII条件を明確にするためC照合順序とCHECKを使い、ULIDは先頭0〜7の正規大文字26文字を検証する。Userの日時はtimestamptzとCURRENT_TIMESTAMPを使う。セッション作成日時・期限・最終アクセス日時は追加しない。ハッシュ保存には[共通Argon2id](password-hashing.md)のPHC形式を使う。NFCは保存前・照合前の入力へ適用し、生パスワードをDBへ保存するカラムは追加しない。DBは現段階ではハッシュ文字列の非空だけを検証する。実行手順と権限は[ローカルPostgreSQL](local-postgresql.md#テーブルmigration)を参照する。以下の旧AI案のSQLファイルや制約を現在の正本とはしない。

### 用途別のDB権限（合意済み）

通常APIと管理操作を分ける大枠に加え、今回の接続用途を通常API・User作成コマンド・goose migrationに分ける方針に合意した。これは接続用途の分担であり、所有者用NOLOGINロール等を含む物理的なロール数を3つに固定するものではない。

| 接続用途 | 許可する操作 | 許可しない操作 |
|---|---|---|
| 通常API | usersのSELECT、auth_sessionsのSELECT・INSERT、#54でDELETEを追加 | User作成・更新・削除、セッション更新、DDL、DBロール管理、所有者への昇格 |
| User作成コマンド | usersのINSERT | 通常の全件読取、既存Userの更新・削除、セッション操作、DDL、DBロール管理 |
| goose migration | 対象スキーマのDDL、gooseの適用済みバージョン管理 | DBロールの作成・管理。通常APIからこの接続情報を使うこと |

初期のDB・ロール作成は別の管理操作とし、APIやUser作成コマンドへsuperuserを渡さない。対象テーブルの所有権と所有者ロールへのmembershipも分け、通常APIがSET ROLE等でDDL権限を得られないようにする。接続に必要なCONNECT・schema USAGE等、具体ロール名・資格情報の注入方法・goose用の所有権は実装時に具体化する。接続情報をそれぞれ用意する手間は増すが、通常APIの不具合等で変更できる範囲を限定できる。

これはPostgreSQLへの接続権限であり、アプリのログインUserの区別とは別物。認証済みUserが自分の情報だけを取得できるAPIの認可も維持する。#54のDELETE追加は[ログアウト保存処理](#issue-54のログアウト保存処理)に従う。#58の再設定専用ロールの権限と索引は[再設定の詳細](password-reset.md#最小db権限と索引)に従う。[PostgreSQLのロール](https://www.postgresql.org/docs/current/user-manag.html)・[権限](https://www.postgresql.org/docs/current/ddl-priv.html)を根拠とする。用途別の分担の合意と、以下のローカル初期設定・テーブル権限の実装範囲を区別する。

現在のローカル初期設定では、接続ロールを `watchbrief_api`・`watchbrief_user_creator`・`watchbrief_password_resetter`・`watchbrief_migrator` とし、migration用が `app` schemaを直接所有する。API・User作成・再設定用は所有者へのmembershipを持たない。手順・秘密ファイル・初期権限は[ローカルPostgreSQL](local-postgresql.md#用途別の初期設定)で管理する。上表のSELECT/INSERTは版1、セッションDELETEは版2のmigrationで付与する。実行した権限試験と未検証範囲はIssueへ記録する。

## Issue #54のログアウト保存処理

**合意済み**：`token_hash` の一意検索を条件に `app.auth_sessions` の対象行を物理削除する。既存の一意索引を利用し、失効状態・時刻等のカラムや索引は追加しない。削除件数0も成功とし、通信断後の再試行を可能にする。DB側の確定を待ってからCookieを削除する。

現在のgooseに `000002_logout_session.sql` を追加し、API用 `watchbrief_api` にauth_sessionsのDELETEを付与する。Downはこの権限のREVOKEだけで、削除済みセッションを復元しない。User作成用ロールのセッション権限、APIのUser更新・DDL・ロール管理権限は追加しない。

これはテーブル単位のDB接続権限であり、DBロールがアプリUserごとの行制限を保証するものではない。HTTP受付で検証したCookieのハッシュだけをユースケース・保存アダプターへ渡し、固定の条件付きSQLで対象を絞る。呼び出し側からUser IDやセッション記録IDを指定して削除するAPIは追加しない。パスワード再設定による全既存セッション削除と競合制御は[再設定の詳細](password-reset.md)を参照する。

## Issue #53の認証物理スキーマ

以下は旧AI案の物理設計を参考として保持したものであり、今回の採用決定ではない。[現在の基本設計案](system-design.md#issue-53の基本設計)に合わせ、実装前に物理型・制約・索引・権限・migrationを再確認する。Vue 3・TypeScript・Vite・PrimeVueの選定による論理テーブル・関係・業務制約の変更はない。その他のテーブルの物理設計も確定しない。DDLの正本は `backend/db/migrations/000001_auth.up.sql`。PostgreSQL 18.6・SQL＋sqlc・pgx/v5を検証対象とし、migrationの実行手順は[ローカル起動](local-development.md)を参照する。

| 保存先 | 型・制約・索引 |
|---|---|
| users.id / auth_sessions.id | `text COLLATE "C"`、主キー、26文字の正規ULIDをCHECK（先頭0〜7、Crockford Base32大文字） |
| users.login_id | `text COLLATE "C" NOT NULL UNIQUE`、`^[a-z0-9_-]{1,64}$` をCHECK。正規化はAPI・コマンドで行う |
| users.password_hash | `text NOT NULL`、80〜512文字をCHECK。Argon2id PHC形式の厳密検査は共通hash処理で行う |
| users.created_at / updated_at | `timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP`。UTC接続設定。User作成時は同値。更新機能は #58で更新する |
| auth_sessions.user_id | `text COLLATE "C" NOT NULL`、users.idへのFK・ON DELETE RESTRICT、非一意B-tree索引 |
| auth_sessions.token_hash | `bytea NOT NULL UNIQUE`、32byteをCHECK。ランダムtoken元byte列のSHA-256。生tokenは保存しない |
| auth_sessions.created_at | `timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP`。認証セッション作成時刻。期限判定には使用しない |

ULIDはGoの `oklog/ulid/v2` とcrypto/randで生成する。整数やUUID型へ変換せずtextで保持し、`char(26)`の空白paddingや独自DB extensionを避ける。主キー・一意制約の索引を重複作成しない。セッションの時間失効・最終アクセス日時は追加しない。ログアウトでの削除／無効化は #54、パスワード再設定の競合制御と更新は #58で具体化する。

| ロール | 付与する権限 |
|---|---|
| watchbrief_owner | NOLOGIN、public schema・table所有者。通常アプリはメンバーにしない |
| watchbrief_migrator | LOGIN、NOINHERIT、ownerのメンバー。明示したmigration transaction内でSET ROLE。DB role作成権限なし |
| watchbrief_api | LOGIN、schema USAGE、users SELECT、auth_sessions SELECT/INSERTのみ。User追加・変更、session削除、DDL、DB role管理は不可 |
| watchbrief_user_admin | LOGIN、schema USAGE、users INSERTのみ。読み取り・DDL・role管理は不可。Web User作成用でありDB管理者とは異なる |
| bootstrap | 専用ローカルcluster初期化とDB/role作成の管理者。資格情報をAPIへ渡さない |

PUBLICからdatabaseのCONNECT/CREATE/TEMPとpublic schema権限を剥がし、必要なCONNECTを個別付与する。全LOGIN roleへNOSUPERUSER/NOCREATEDB/NOCREATEROLE/NOREPLICATION/NOBYPASSRLSを明示する。migrationは番号付きSQLをpsqlで明示適用し、schema_migrationsの版・DDLを同じtransactionで確定する。APIの起動時にDDLを実行しない。権限・制約の試験SQLは `backend/db/verify.sql`。実行状況は[検証記録](verification-issue-53.md)を参照する。

## Issue #56のCollection物理設計

委任に基づく実装判断として、既存論理モデルのCollection・CollectionSchedule・WatchTarget・InformationSourceをgoose版3へ具体化した。IDは既存と同じtext/C照合・正規ULID CHECK、現在設定の親子はCASCADE、User所有者はRESTRICT。Scheduleのcollection_idは一意。初回lookbackはaiなら日数NULL、daysなら1〜365をCHECKし、NULLによる条件すり抜けも拒否する。名前・cron・URLは非空text、任意の指示は既定空文字、enabledは既定false、display_orderはintegerで一意制約なし。設定4種の作成・更新日時はtimestamptz。

索引はcollections(user_id,display_order,id)、watch_targets(collection_id,display_order,id)、information_sources(watch_target_id,id)。Scheduleの一意索引と主キーは重複作成しない。URLの一意制約は追加しない。通常APIに4テーブルのSELECTだけを付与し、User作成ロールの権限を増やさない。旧migrationは編集せず、API起動前の明示migrationを維持する。

設定をONにするには有効対象とScheduleが必要という業務ルールは維持する。今回編集APIはなく、管理seedは常に停止中Collectionと有効対象・Scheduleを原子的に作る。#18で編集経路を追加する際には、同じトランザクションでのON条件・停止・削除条件を実装・検証する。#57の履歴への外部キーは今回先取りしない。版3のDownは現在設定の全削除になるため、通常操作・検証では実行しない。

## 保存Reportの物理化（Issue #57）

委任に基づく実装判断として `000004_saved_reports.sql`・`000005_immutable_history_links.sql`・`000006_active_job_collection.sql` でJob・Attempt・Reportと対象結果・事実・出典・事実出典の対応・取得失敗を物理化する。既存migrationは変更しない。識別子は既存と同じULID形式の `text COLLATE "C"`、時刻は `timestamptz`、入力snapshotはobject型のJSONBとする。履歴上のtarget_id/source_idはULID検証のみを行い、現在設定への外部キーを持たない。

Job.user_idはNOT NULL/RESTRICT、Collection参照はSET NULL（NULLは完了履歴のみ許可）、履歴内のFKはRESTRICTとする。本文・対象名等の必須値、状態、失敗分類、started≤heartbeat≤finished、登録元source_idの有無をCHECKで制限する。Attemptは初期MVPの1Job最大1件をUNIQUEとattempt_number=1で制約し、将来retry時はmigrationで変更する。事実区分は `current_state` / `update` とする。対象結果statusの業務上の意味は既存の3状態（`重要な変化あり`・`重要な変化なし`・`調査できませんでした`）とする。今回のfixture用の実装判断として日本語値 `重要な変化あり` を保存する。DBは非空文字列を制約し、一般的な保存enumと生成時の検証方式はWorker実装時に具体化する。

成功AttemptとReportの1対1、JobとAttemptの状態一致、queued時のAttempt不在、queued_at≤started_at、updates_started_at≤started_atはトランザクション末尾の遅延制約triggerで確認する。これにより成功状態とReportを同じトランザクション内で順序を問わず保存でき、片側だけをcommitできない。Job/Attempt/ReportのID、Attempt→Job・Report→Attemptの親参照、試行番号、Jobの所有者・名前・入力snapshot・予定時刻・登録時刻の変更、Collection参照の付け替えと、実行前/実行中Collection参照の解除はtriggerで拒否する。事実と出典の対象一致は既存方針どおり書込側が検証する。現在の書込経路は固定した対応関係を持つ管理seedのみである。

一覧は所有User・予定時刻・ID、処理対象は状態・予定時刻・ID、Collection削除判定はCollection・状態の索引を使う。配下の対象・事実・出典・失敗の取得索引、出典から対応表への逆引き索引も作る。APIは新しい8テーブルをSELECTできるが、INSERT/UPDATE/DELETE/DDL権限は持たない。

テーブル横断条件をCHECKに埋め込まず、[PostgreSQL公式の制約](https://www.postgresql.org/docs/18/ddl-constraints.html)と[遅延制約trigger](https://www.postgresql.org/docs/18/sql-createtrigger.html)の性質に従う。生成処理・監視・自動retryをこのmigrationで実装したとは扱わない。

管理seedの現行fixtureはv3で、対象状態、出典付きcurrent_stateとupdate、本文、入力snapshotを整合させる。snapshotはschema_version、調査方針、初回確認方針、有効対象・情報源の履歴値を含む。検証途中の旧v1/v2履歴は入力不変の方針に従って非上書きで保持するが、現行seedの参照例にはしない。新規環境ではv3だけを追加する。

取得失敗の本文表示は正本のルールに従って情報源の名称・URLだけとし、reasonは構造化行に保持して本文へ含めない。
