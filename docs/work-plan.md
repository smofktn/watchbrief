# WatchBrief 作業計画と将来候補の追跡

## この一覧の位置付け

既存Issueと設計文書から、初期実装への引き継ぎと将来候補を整理した一覧である。棚卸しの出典はIssue #1・#3・#7・#9・#10・#11・#14の本文・コメントと各設計文書。旧Issue #2・#4・#5・#6・#8は廃止経緯を確認した。

ここは作業の追跡先と進行順の提案をまとめる。機能仕様の正本は[プロダクト仕様](product-definition.md)。Issue起票は候補の採用・MVPへの追加・着手順の合意ではない。AWS作成・課金・外部公開は別途合意してから扱う。Issueのclose/reopenは利用者の明示指示時だけ行う。

Issue本文の古い表現より、後の合意コメントと正本文書を優先する。ブランチ・コミット・PRの状態はGitHubと作業ツリーで都度確認する。

## Issue #1の整理状況と完了条件

初期ユーザー作成・再設定の骨格と、再設定時の全既存セッション無効化に合意した。パスワード更新と無効化を一体で確定し、具体ハッシュ・コマンド・入力・競合制御は #17 へ引き継ぐ。

ログイン失敗制限と認証専用記録は、利用者の指示により初期MVPでは作り込まず #49 へ回す。既存の認証・認可・エラーログと秘密情報非出力は維持する。

初期MVPでは内部・メタデータ宛ての取得を拒否するSSRF対策を行い、名前解決後の実接続先と転送先にも適用する大枠に合意した。具体ライブラリ・設定・異常系と外部サービス側の制御範囲は #20、取得拒否・部分失敗は #7 へ引き継ぐ。公開サイト自体の悪意を判定する機能は将来対応 #50 とする。Cookie・CSRFの具体方式は合意どおり実装時の詳細設計へ残す。

大枠の議題は整理した。Secret保管・注入の初期案と早期移行意向は未決定範囲を明記して引き継ぎ、正常フロー・分類・権限・交換／漏えい対応・後続Issueの整合を確認して設計PRにまとめる。実装・AWS構築・全詳細値の決定をこのIssueの完了条件へ加えない。

残る作業は設計文書・引き継ぎの最終確認とPRレビューであり、追加の大枠相談は予定していない。レビューで仕様変更が必要になれば未決定として相談する。Issueを閉じる操作は利用者の明示指示時だけ行う。

## 実装開始までの推奨順（未合意の計画案）

1. #1 の設計文書と引き継ぎをレビューする。
2. #17 で最小限の詳細設計と技術選定を行い、ローカルで「ログイン→自分の設定・保存済みReport取得→ログアウト」を実装・検証する。架空データで確認し、Report生成やAWS構築を前提にしない。
3. #18 の設定編集・閲覧へ広げる。#7 で非同期処理とログ・障害時の全体像を揃え、#19 のScheduler→Worker→DB→Report閲覧を検証用応答でつなぐ。
4. #20 で外部AI・検索・取得方式を比較して接続する。料金の発生する検証は別途確認する。
5. #21・#22 で初期EC2配備と運用を具体化する。構成・料金・公開の合意後に作成・検証する。

#7の全設定値や #23以降の将来候補をすべて完了するまで、#17の実装開始を待たせる計画にはしない。ただし各実装で必要な認証・入力検証・整合性・秘密情報の扱いは、その部分の実装と合わせて設計・確認する。最初の正常系を通しただけで異常系・公開準備が完了したとは扱わない。

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
