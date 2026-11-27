  # WatchBrief 作業計画と将来候補の追跡

## この一覧の位置付け

既存Issueと設計文書から、初期実装への引き継ぎと将来候補を整理した一覧である。棚卸しの出典はIssue #1・#3・#7・#9・#10・#11・#14の本文・コメントと各設計文書。旧Issue #2・#4・#5・#6・#8は廃止経緯を確認した。

ここは作業の追跡先と進行順の提案をまとめる。機能仕様の正本は[プロダクト仕様](product-definition.md)。Issue起票は候補の採用・MVPへの追加・着手順の合意ではない。AWS作成・課金・外部公開は別途合意してから扱う。Issueのclose/reopenは利用者の明示指示時だけ行う。

Issue本文の古い表現より、後の合意コメントと正本文書を優先する。ブランチ・コミット・PRの状態はGitHubと作業ツリーで都度確認する。

## Issue #1の整理状況と完了条件

初期ユーザー作成・再設定の骨格と、再設定時の全既存セッション無効化に合意した。パスワード更新と無効化を一体で確定し、具体ハッシュ・User作成は#53、再設定CLI・競合制御は[再設定の詳細](password-reset.md)を参照する。本番の管理接続・運用検証は構築時に扱う。

ログイン失敗制限と認証専用記録は、利用者の指示により初期MVPでは作り込まず #49 へ回す。既存の認証・認可・エラーログと秘密情報非出力は維持する。

初期MVPでは内部・メタデータ宛ての取得を拒否するSSRF対策を行い、名前解決後の実接続先と転送先にも適用する大枠に合意した。具体ライブラリ・設定・異常系と外部サービス側の制御範囲は #20、取得拒否・部分失敗は #7 へ引き継ぐ。公開サイト自体の悪意を判定する機能は将来対応 #50 とする。Cookie・CSRFの具体方式は合意どおり実装時の詳細設計へ残す。

大枠の議題は整理した。Secret保管・注入の初期案と早期移行意向は未決定範囲を明記して引き継ぎ、正常フロー・分類・権限・交換／漏えい対応・後続Issueの整合を確認して設計PRにまとめる。実装・AWS構築・全詳細値の決定をこのIssueの完了条件へ加えない。

残る作業は設計文書・引き継ぎの最終確認とPRレビューであり、追加の大枠相談は予定していない。レビューで仕様変更が必要になれば未決定として相談する。Issueを閉じる操作は利用者の明示指示時だけ行う。

## 実装開始までの推奨順（未合意の計画案）

1. #1 の設計文書と引き継ぎをレビューする。
2. #17 は下記の子Issueへ分けて進める。最初は #53 の「架空User作成→ログイン→本人情報表示」までを実装・検証し、その後にログアウト、元URLへの復帰、設定・保存済みReport取得、再設定を足す。架空データで確認し、Report生成やAWS構築を前提にしない。
3. #18 の設定編集・閲覧へ広げる。#7 で非同期処理とログ・障害時の全体像を揃え、#19 のScheduler→Worker→DB→Report閲覧を検証用応答でつなぐ。
4. #20 で外部AI・検索・取得方式を比較して接続する。料金の発生する検証は別途確認する。
5. #21・#22 で初期EC2配備と運用を具体化する。構成・料金・公開の合意後に作成・検証する。

#7の全設定値や #23以降の将来候補をすべて完了するまで、#17の実装開始を待たせる計画にはしない。ただし各実装で必要な認証・入力検証・整合性・秘密情報の扱いは、その部分の実装と合わせて設計・確認する。最初の正常系を通しただけで異常系・公開準備が完了したとは扱わない。

## Issue #17の子Issueと最初の確認範囲（合意済み）

利用者と、小さく動作確認できる単位で進めることに合意した。#17は全体の追跡先として維持し、最初の完了範囲を全体フローから #53へ絞る。各子Issueで必要な詳細設計→実装→確認を行い、後続の詳細をすべて決めてから着手する進め方にはしない。

| 順序 | 子Issue | 確認する動作 |
|---|---|---|
| 1 | [#53](https://github.com/smofktn/watchbrief/issues/53) | 架空Userを管理コマンドで作成し、ログインして本人情報を表示する |
| 2 | [#54](https://github.com/smofktn/watchbrief/issues/54) | ログアウト後、同じセッショントークンの利用を拒否する |
| 3 | [#55](https://github.com/smofktn/watchbrief/issues/55) | 保護URLからログインへ遷移し、成功後に安全な元URLへ戻る |
| 4 | [#56](https://github.com/smofktn/watchbrief/issues/56) | 自分の架空Collection設定を取得・表示し、他Userの設定は取得させない |
| 5 | [#57](https://github.com/smofktn/watchbrief/issues/57) | 自分の保存済みReportを取得し、他Userは404、Collection削除後も認可を維持する |
| 6 | [#58](https://github.com/smofktn/watchbrief/issues/58) | パスワード再設定後に全既存セッションを拒否し、同時処理でも失効がすり抜けないことを確認する |
| 7 | [#59](https://github.com/smofktn/watchbrief/issues/59) | DB障害・復旧とログ非混入を横断確認し、全体の起動・検証手順をまとめる |

Issue #53の既存AI実装案は[認証の実装方式](auth-implementation.md)、その起動案は[ローカル手順](local-development.md)、過去の検証状況は[担当記録](verification-issue-53.md)を参照する。参考として残す未完成の案であり、利用者の採用決定・今回の実装成果とは区別する。

認証・認可・CSRF・入力検証・整合性・秘密情報保護は各処理の実装時から扱い、最後の横断確認へ延期しない。必要な依存・完了条件は各子Issueで管理する。#53でログイン・本人情報取得を通しただけで、#17全体を完了したとは扱わない。

### 利用者主体で子Issueを進める運用（合意済み）

今回の #17完了作業では、利用者の明示依頼により残る子Issueをサブエージェントへ順次委任し、実装・セルフレビュー・独立レビューと修正・親向けPR・親へのマージまで進める。通常フォルダーを使い、同時に複数担当が編集しない。既存仕様の変更・未採用候補の一括採用を承認するものではない。Issue状態変更と #17からmainへのマージは今回の委任範囲に含めない。以下は一般の相談時の進め方として維持する。

以前のCodex担当・レビューセッションを起動する運用と、その後の利用者がコードを書く運用は、以下の進め方へ更新する。

- API設計・画面設計・アーキテクチャは利用者とAIが一緒に検討する。全体像を短く示した後、一度に1つの議題を扱い、提案と合意済みを区別する。
- 合意した範囲の実装・テスト・セルフレビューはAIが担当し、実際の動作を確認してから次へ進む。通常の実装上の細部は理由を説明して判断し、利用者体験や合意済み仕様を変える判断は相談する。未合意の工程をまとめて実装したり、別エージェント・Codex担当・tmux監督処理を起動したりしない。
- 通常の作業フォルダーでブランチを切り替え、worktreeは使わない。#17の子Issueは必要な成果をコミットした#17を起点にブランチを作る。未コミット変更はブランチの履歴に含まれないため、分岐前に内容と基点を確認する。
- #17の全体管理・レビューを扱うメインセッションと、子Issueの支援セッションを区別する。同じ作業フォルダーで同時に編集やブランチ切替をしない。
- 実装後はセルフレビューし、指摘修正と必要な再検証を行う。実動作確認済み・文書のみ確認・未確認を分け、未解決を完了扱いにしない。
- 明示された継続委任では、窓口が正本・未完了範囲から次の小子Issueを選定して担当へ指示し、子Issueごとに設計→設計セルフレビュー/修正→実装/テスト→実装・テストのセルフレビュー/修正→必要な再テスト→Issue全体セルフレビュー→窓口独立レビュー→親への統合→窓口選定の次Issueを順に進める。これは1件ずつのサイクルであり、実装担当が複数Issueを並行/先行着手する許可ではない。継続委任は正本にない機能・要件変更・外部サービスの採用、main統合、Issue状態変更、AWS/通常DB操作を許可しない。
- 決定は担当docs、経緯・進捗・検証結果はIssue・PRへ残す。既存AI案は参考として残し、具体方式を利用者の採用決定として扱わない。実行ログ、セッションID、制御用ファイルはGitへ追加しない。
- Issueのclose/reopen・PRマージは引き続き利用者の明示指示時だけ行う。

設定編集・本格的なReport閲覧は #18、生成処理は #19、外部接続は #20、AWS配備は #21、運用監視は #22、ログイン試行制限・認証専用記録は #49へ分ける。機能仕様・業務ルールを変更する分割ではなく、技術の具体方式は必要な段階で詳細化し、採用理由と影響する設計へ記録する。

現在のローカル構築・正常経路・実DB停止と復旧の横断検証は[起動・確認手順](verification-issue-17.md)を参照する。手順と実行結果を区別し、結果はIssue/PRへ記録する。

## PRの自動検証（Issue #70・合意済み）

[#70](https://github.com/smofktn/watchbrief/issues/70)は、#17・#68の既存Make入口をGitHub Actionsへ接続する最小CIを担当する。main向けPRで依存準備→テスト・型検査・差分検査→ビルドを実行し、GitHub上で結果を確認する。runtimeの正本は `.tool-versions`、利用手順と対象外は[CI手順](../CONTRIBUTING.md#ciissue-70)を参照する。実行結果はIssue・PRに記録する。

実DB・PTY・Chromium E2EのCI化やcoverageの強制は含めない。手動配備・HTTPS・復旧は #21、手動手順を確立した後の配備自動化は #29に分ける。今回のCI採用は #29の方式採用やAWS作成・課金・公開の承認を意味しない。

## 初期実装・構築への引き継ぎ

| Issue | 対象 | 主な入力 |
|---|---|---|
| [#17](https://github.com/smofktn/watchbrief/issues/17) | 実装：Vue・Go・PostgreSQLでログインからReport取得・ログアウトまで通す | docs/system-design.md のIssue #1認証フロー、docs/database-design.md の認証セッション |
| [#18](https://github.com/smofktn/watchbrief/issues/18) | 実装：コレクション設定の編集と保存済みReportの閲覧をつなぐ | docs/product-definition.md の画面構成、docs/database-design.md |
| [#19](https://github.com/smofktn/watchbrief/issues/19) | 実装：Scheduler・Worker・停止監視による生成Jobの一連の処理を通す | docs/domain-model.md の状態遷移、docs/system-design.md の生成フロー |
| [#20](https://github.com/smofktn/watchbrief/issues/20) | 実装準備・実装：外部AIとWeb検索・情報取得の方式を比較して接続する | docs/technical-direction.md のAI・情報収集、docs/product-definition.md の自動生成 |
| [#21](https://github.com/smofktn/watchbrief/issues/21) | 構築：初期EC2構成への配備・HTTPS・手動更新と復旧を検証する | docs/system-design.md の初期配置・構築順・運用設計案 |
| [#22](https://github.com/smofktn/watchbrief/issues/22) | 実装・運用：非公開ログ・監視・障害調査と停止再開を整える | docs/system-design.md のログ・監視・障害対応、docs/product-definition.md のエラー方針 |

#1は認証・公開境界、#7はJob・ログ・復旧の設計を引き続き担当する。バージョン・ORM・migration・queue・ブラウザの取得／cache設定等は上記Issueの必要な段階で決め、技術候補ごとの採用Issueは重複して作らない。

## Issue #21の初回配備範囲（合意済み）

初回はCollection本体の作成・編集までを含む版を使い、HTTPSのログインから保存・再表示・ログアウトを通す。[利用者向けの確認範囲](product-definition.md#ec2初回配備の確認範囲issue-21合意済み)と[統合・配備・運用の進め方](system-design.md#issue-21の初回配備の進め方合意済み)を参照する。#18の残機能、#19・#20の完成を初回の前提にせず、#21全体に残るJob生成の確認とは区別する。マージ・課金・外部公開はそれぞれ別途明示指示・合意後に実施する。

## 将来の改善・移行候補

| Issue | 対象 | 状態 |
|---|---|---|
| [#23](https://github.com/smofktn/watchbrief/issues/23) | 将来移行：SecretをEC2の保護ファイルからSecrets Managerへ移す | 早期移行の意向 |
| [#24](https://github.com/smofktn/watchbrief/issues/24) | 将来移行：同居PostgreSQLを非公開RDSへ分離する | 将来の方向性に合意 |
| [#25](https://github.com/smofktn/watchbrief/issues/25) | 将来移行：SSHからSession Managerへ管理接続を切り替える | 早期移行の方向性に合意 |
| [#26](https://github.com/smofktn/watchbrief/issues/26) | 将来移行：アプリをコンテナ化しECS等の実行先へ移す | 将来の方向性に合意 |
| [#27](https://github.com/smofktn/watchbrief/issues/27) | 将来整備：DBバックアップ・復元手順と復元試験を用意する | 将来整備の方向性・本運用前に見直し |
| [#28](https://github.com/smofktn/watchbrief/issues/28) | 将来整備：Report本文を非公開Gitへ副次アーカイブする | 将来整備の方向性に合意 |
| [#29](https://github.com/smofktn/watchbrief/issues/29) | 将来改善：手動デプロイの手順確立後に配置・確認を自動化する | 候補・方式未採用 |
| [#30](https://github.com/smofktn/watchbrief/issues/30) | 技術検討：初期構成の再現手順とIaCの導入時期・方式を決める | 候補・導入時期未決定 |
| [#31](https://github.com/smofktn/watchbrief/issues/31) | 将来改善：生成Jobの自動retryと再実行ルールを設計する | MVP後の優先改善候補 |
| [#32](https://github.com/smofktn/watchbrief/issues/32) | 将来検討：メールによるパスワードリセットを提供する | 機能候補・採用未決定 |
| [#33](https://github.com/smofktn/watchbrief/issues/33) | 将来検討：時間によるセッション失効と複数端末ログアウト管理 | 機能候補・採用未決定 |
| [#34](https://github.com/smofktn/watchbrief/issues/34) | 将来検討：ホームに次回の生成予定時刻を表示する | 機能候補・採用未決定 |
| [#35](https://github.com/smofktn/watchbrief/issues/35) | 将来検討：通常の履歴・確認期間を変えないテスト生成と手動生成の要否 | 機能候補・採用未決定 |
| [#36](https://github.com/smofktn/watchbrief/issues/36) | 将来検討：AIによるReportタイトルを付ける | 機能候補・採用未決定 |
| [#37](https://github.com/smofktn/watchbrief/issues/37) | 将来検討：事実ごとの出来事日付をReportへ表示する | 機能候補・採用未決定 |
| [#38](https://github.com/smofktn/watchbrief/issues/38) | 将来検討：Reportを通常一覧から隠すアーカイブを追加する | 機能候補・採用未決定 |
| [#39](https://github.com/smofktn/watchbrief/issues/39) | 将来検討：Report内の用語まとめと利用者の手動メモ | 機能候補・採用未決定 |
| [#40](https://github.com/smofktn/watchbrief/issues/40) | 将来検討：生成Attempt履歴・失敗詳細と情報源の技術情報を画面表示する | 機能候補・採用未決定 |
| [#41](https://github.com/smofktn/watchbrief/issues/41) | 将来改善：情報源URLの正規化と同一対象内の重複検査を検討する | 問題発生時の改善候補 |
| [#42](https://github.com/smofktn/watchbrief/issues/42) | 早期改善候補：編集途中の未保存変更を破棄する前に確認する | 早期改善候補・採用時期未決定 |
| [#43](https://github.com/smofktn/watchbrief/issues/43) | 将来検討：予定とJobを比較して生成の実行漏れを画面に表示する | 必要性が生じた場合の候補 |
| [#44](https://github.com/smofktn/watchbrief/issues/44) | 将来検討：Userと生成履歴の削除・保持期間を安全に設計する | 必要性が生じた場合の候補 |
| [#45](https://github.com/smofktn/watchbrief/issues/45) | 将来検討：Reportの全文検索・高度なフィルタとコレクション横断表示 | 機能候補・優先度未決定 |
| [#46](https://github.com/smofktn/watchbrief/issues/46) | 将来検討：設定変更等を追跡する独立した監査記録 | 必要性が生じた場合の候補 |
| [#47](https://github.com/smofktn/watchbrief/issues/47) | 将来検討：Report生成完了・失敗をメールやPushで利用者へ通知する | 初期対象外・採用未決定 |
| [#48](https://github.com/smofktn/watchbrief/issues/48) | 将来検討：Report等のエクスポート・大きな添付物とS3利用の要否 | 機能・保存先候補・採用未決定 |
| [#49](https://github.com/smofktn/watchbrief/issues/49) | MVP後の改善：ログイン試行制限と認証イベント記録 | 利用者の指示により初期MVPの作り込みから延期 |
| [#50](https://github.com/smofktn/watchbrief/issues/50) | 将来対応：悪意ある外部サイトの判定と取得拒否 | 将来機能として希望あり。具体方式・サービス・費用は未決定 |

既存 #14 はインフラ移行全体の親として維持し、#24・#25・#26を具体作業の追跡先とする。Secrets Manager早期移行の意向は#23、バックアップは#27、Git本文アーカイブは#28へ分けた。独立した改善は別Issueにし、同じ文脈の用語まとめ・メモ、失敗詳細・Attempt表示、User／履歴の削除等はIssue内でそれぞれ採否を判断する。

## 出典と対応範囲

| 出典の将来項目・引き継ぎ | 追跡先 |
|---|---|
| メール再設定、時間失効・端末ログアウト | #32・#33。管理コマンドの初期運用・Cookie／CSRFは#1→#17 |
| ログイン失敗制限・認証専用イベント記録 | #49。初期MVPの必須実装から外し、認証・認可と既存のエラーログは#17・#7で維持 |
| 悪意ある外部サイトの判定・取得拒否 | #50。初期MVPで実装する内部宛て拒否のSSRF対策とは分けて将来検討 |
| 次回予定、テスト・手動生成、AIタイトル、事実日付、画面上のアーカイブ、用語・メモ | #34〜#39 |
| 自動retry、Attempt履歴・失敗技術情報、URL重複・正規化、未保存変更確認 | #31・#40〜#42 |
| Job未作成の表示、User／履歴保持・削除、検索・高度フィルタ・横断表示、監査記録、利用者通知 | #43〜#47 |
| DBバックアップ、非公開Git本文アーカイブ、export・大きな添付物等のS3候補 | #27・#28・#48 |
| Secrets Manager、RDS、Session Manager、コンテナ／ECS | #23〜#26、親#14 |
| 手動配備後の自動化、IaC導入判断 | #29・#30。最初の手動配備・再現手順は#21 |
| バージョン・配置・DB物理設計・ORM・migration・権限・管理コマンド | #17・#18・#21。必要な部分から詳細化 |
| queue・cron・timeout・heartbeat・停止検知・失敗分類・ログ／監視・retry可否 | #7→#19・#22。実際の自動retry導入は#31 |
| AI・検索・構造化出力・出典・SSRF・生成費用・外部送信条件 | #20。秘密情報・公開境界は#1を入力にする |

廃止済みのPages／VitePress・静的ファイル中心の生成、旧Basic認証、初期SQLite・初期RDS、取消Job、開始時の最新設定利用、共用親ドメイン確定等は、後の合意で更新されたため再起票しない。会員登録・ユーザー管理画面・公開Report・投資専用機能等の「対象外」としか示されていないものは、将来の採用意向と解釈しない。Raspberry Piの話は他リポジトリでの学習活用の背景であり、WatchBriefの作業Issueへ追加しない。

## 確認状況の扱い

この一覧は設計と追跡先を示し、実装・構築済みを意味しない。文書検査やレビューの実施結果は対象PRへ記録する。アプリ・AWS・移行・復旧の実動作は未確認であり、各担当Issueで検証する。

## Issue #18の最初の確認範囲（合意済み）

最初はCollection本体の新規作成を通す。入力範囲と利用者向けの振る舞いは[プロダクト仕様](product-definition.md#collection本体の新規作成issue-18の最初の範囲合意済み)、具体画面は[新規作成](画面仕様/コレクションの新規作成.md)、保存契約は[作成API](API仕様/post-api-collections.md)を参照する。結果不明時にも同じ要求・入力の再試行で重複作成しない方式に合意した。具体契約は作成APIで管理する。

最初のCollection本体作成と重複しない再試行は、子Issue [#72](https://github.com/smofktn/watchbrief/issues/72)として独立したまとまりで扱うことに合意した。ブランチはmain→#18→#72、子PRのbaseは#18とする。#18は全体の追跡先として維持し、最初の経路だけで完了扱いにしない。

利用者の依頼に基づき、次の検討対象として子Issue [#74](https://github.com/smofktn/watchbrief/issues/74)「既存Collection本体の編集・保存・再表示」を選定した。最初の正常系は、作成済みの停止中Collectionで名前・調査方針・初回確認期間を変更し、保存後の詳細・一覧・再読み込みで確認する経路とする。競合時は入力を保持して保存を止め、最新値を確認して編集し直す方針に合意した。版番号と更新要求番号で競合と同一保存操作の再試行を区別する方式にも合意し、本体編集の実装・検証を進める。具体契約は[更新API](API仕様/patch-api-collections.md)と[編集画面](画面仕様/コレクション本体の編集.md)へ記録する。現在の合意範囲は[本体編集の仕様](product-definition.md#collection本体の編集issue-74)を参照する。

その後は、配下設定の編集、有効化・削除・並べ替え、履歴・ホームの閲覧、安全なMarkdown表示を動作確認できるまとまりで進める案とする。後続工程の一括実装や、残りの子Issue・PRの分け方はまだ合意していない。

## Issue #18の次の確認範囲（範囲合意済み）

停止中Collection配下のWatchTargetの対象名・任意の補足指示の追加と編集を扱う。[合意範囲と未決定事項](product-definition.md#停止中collectionのwatchtarget追加編集issue-18範囲合意済み)を参照する。画面・API・入力範囲と子Issue #78に合意し、本体と対象の一括保存を実装する。子PRは既存方針どおり#18をbaseとする。

合意したWatchTarget一括保存は子Issue [#78](https://github.com/smofktn/watchbrief/issues/78)で実装・検証・セルフレビューする。親#18の設計commitから子#78を作り、PRのbaseは#18とする。

## 情報源追加・編集（Issue #18）

代理回答で停止中Collectionの情報源追加/編集・本体/対象との一括保存を採用した。親#18に具体契約を記録してcommit後、子Issue/ブランチで実装・隔離DB/画面検証・セルフレビューを進める。子PRのbaseは#18。今回だけで親全体の完了とはしない。

入力・保存契約は[正本](product-definition.md#停止中collectionの情報源追加編集issue-18)と[更新API](API仕様/patch-api-collections.md#情報源追加編集への拡張issue-18)を参照する。

子Issueは[#80](https://github.com/smofktn/watchbrief/issues/80)。親設計commitから子ブランチ#80を作り、子PRのbaseを#18とする。

## 停止中CollectionのSchedule追加・編集（Issue #82）

利用者から設計回答を委任された上ペインの代理判断で採用。既存編集に日本時間raw cronを追加し、本体・対象・情報源と一括保存する。詳細→追加→保存→再読込→式編集→保存/再表示を最小正常系とし、全体版・旧要求互換・原子性・入力保持・認証消去を維持する。契約は[更新API](API仕様/patch-api-collections.md#schedule追加編集)、操作は[編集画面](画面仕様/コレクション本体の編集.md#schedule追加編集issue-82)。

削除・有効化等は#18の後続、次回予定表示は#34、生成/実行は#19、AI/外部取得は#20。#18全体の完了を意味しない。通常DB/本番/EC2への版13適用と配備は別途扱う。

## 保存済みReportの安全なMarkdown表示

利用者がこの既存ペインだけへ実装・検証・セルフレビュー/修正・親#18向けPR提出までを明示依頼。上ペインの設計代理判断に基づき、所有Report深いURL→ログイン復帰→見出し/段落/強調/リスト/コード/出典リンク→再読込→logoutを通す。本格一覧/home・生成/外部取得・Collection有効化は別作業。#18全体は未完了。

具体的な表示契約は[画面仕様](画面仕様/保存済みReportの取得.md#安全なmarkdown表示)を参照する。

この表示作業は子Issue [#84](https://github.com/smofktn/watchbrief/issues/84)。親の設計commit起点の子ブランチで実装し、PR base=#18とする。

## 保存済み成功Report一覧の追加取得

利用者の継続委任と上ペインの代理判断で、成功Report一覧の30件ずつcursor追加取得を小さな子作業とする。35件→30件→残り5件/終了→安全な詳細→一覧/再読込→logoutを先に通し、境界/精度/失敗復旧/認証を隔離検証する。子PR base=#18、担当ペインはマージ/Issue状態変更をせず監督レビューへ提出する。#18全体は未完了。

具体契約は[取得API](API仕様/get-api-reports.md#30件ずつのcursor追加取得)を参照する。

追加取得の子Issueは[#86](https://github.com/smofktn/watchbrief/issues/86)。親の設計commitを起点に子ブランチで実装し、監督レビュー用PRを#18へ提出する。

## 日付別の保存済み予定・結果一覧（Issue #88）

利用者の直接了承に基づく[#18の採用記録](https://github.com/smofktn/watchbrief/issues/18#issuecomment-7208655478)を正本へ反映し、親設計commitから子#88を作る。既存レポート→日付別30件→続き→成功詳細→更新/再読込→logoutを先に通し、混合4状態・境界/精度/JST・所有者/削除済みCollection・障害保持/復旧・認証/遅延破棄を使い捨てCompose/Chromiumで検証する。

唯一の実装担当がセルフレビュー/修正後にbase=#18の子PRを提出し、窓口が独立レビューと親への統合を担当する。実装担当はマージ/Issue状態変更をしない。親#18全体は未完了。通常DB/AWS/元AWSフォルダーの変更は対象外。具体契約は[API](API仕様/get-api-generation-jobs.md)と[画面](画面仕様/日付別の予定・結果一覧.md)。

## Collection別の保存済み予定・結果一覧（Issue #90）

既存のCollection別履歴仕様を `/reports/collections` の選択と `/reports/collections/{ULID}` の履歴閲覧へつなぐ。本人の現在Collectionだけを選び、選択Collectionの保存済みJobを30件ずつ新しい順に取得する。成功だけ既存詳細へ移動する。API/画面/認可/cursor/削除競合の具体契約は[API](API仕様/get-api-collection-generation-jobs.md)・[画面](画面仕様/Collection別の予定・結果一覧.md)・関連設計を参照する。

親#18へ設計commitを反映した後、そのcommitを起点に子#90を作り、PR baseを#18とする。Collection候補→履歴→成功詳細→戻る/再読込→logoutを最小正常系として、隔離DB/架空データ/Chromiumで境界・認可・削除・切替競合・失敗保持を検証する。home、生成・外部取得、Collection編集/有効化/削除/並べ替えは範囲外。窓口が独立レビュー・親への統合を行い、実装担当はmerge・Issue状態変更を行わない。親#18全体は未完了。

## ホームの最新保存済みReport（Issue #92）

親#18の設計commitを反映後、そのcommitから子#92を作り、PR baseを#18とする。ホームに本人の現在Collectionを既存順で表示し、最新保存済み成功Reportがある場合だけ既存詳細へリンクする。ホームからReport詳細→再読込/手動更新→logoutを最小正常系とし、所有者・空/未生成・最新選択・改名/削除・失敗復旧・認証/遅延破棄を隔離環境で検証する。

当日予定状態は小Issue #94で扱い、並べ替え・Collection削除/有効化は親#18の後続、次回予定は#34とする。生成/外部アクセス/過去Job復元は行わない。親#18全体は未完了。契約は[API](API仕様/get-api-home.md)・[画面](画面仕様/ホーム.md)と関連設計を参照する。

## ホームの本日予定状態（Issue #94）

現在CollectionごとにJST本日の保存済みGenerationJobを4状態別に数え、ホームの最新Report導線へ補助表示する。Jobが0件なら状態欄を出さず、既存のReportなし表示は独立して維持する。時刻境界・JSON・表示細部は正本と[API](API仕様/get-api-home.md)・[画面](画面仕様/ホーム.md)に従う。

並べ替え・Collection削除/有効化、次回予定#34、生成・外部取得・cronからの過去Job復元は対象外。実装は親#18の子Issue/PRとして扱い、親全体を完了扱いにしない。

## ホームからのCollection表示順変更（Issue #96）

ホームで現在の本人Collectionを一括並べ替えし、成功後にホーム・既存設定一覧・再読込へ反映する。保存要求は完全な現在順/versionを前提条件にし、owner lock namespace 18と同一transaction内の要求digest/replay・一括更新で競合、応答喪失、部分保存を扱う。表示順一意制約なし、同値時ID順、既存versionと要求互換を維持する。API・画面の詳細契約は[get API](API仕様/get-api-home.md)、[順序更新API](API仕様/put-api-collection-order.md)、[ホーム](画面仕様/ホーム.md)を参照する。

ON/OFF、最新Report導線、本日状態、既存一覧/詳細、作成/編集と認証epochの消去を保つ。Collection削除機能は現行にないため追加せず、将来実装時はowner lockを共有する。#18は未完了。
