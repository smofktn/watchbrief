# Frontend

Vue・TypeScript・Vite・PrimeVueによるログインフォーム。入力をGo APIへ送り、400の入力不正を入力欄の近くへ、内部障害・通信失敗を別の文言で表示する。入力値・応答本文をログや永続ストレージへ保存せず、画面のメモリで扱う。認証とCookie発行はGo APIに接続済み。ログイン後と画面の初期表示・再読み込み時に本人情報を取得してログインIDを表示する。画面の設計は[画面仕様](../docs/画面仕様/ログイン・本人情報表示.md)、通信契約は[API仕様](../docs/API仕様/README.md)で継続して管理する。

リポジトリルートの `.tool-versions` に従いasdfでNodeを導入した後、このディレクトリで実行する。

DB・API・画面をまとめて起動する場合は、初期設定・明示migration後にリポジトリルートで `make dev` を使う。Ctrl+CでAPIと画面を終了し、DBは残す。詳細は[ルートの起動手順](../README.md#開発コマンド)を参照する。このディレクトリの `make dev` は従来どおり担当アプリだけを起動する。

```bash
make install
make dev
```

`make help` で一覧を表示する。ルートからは `make frontend-dev`・`make frontend-build` 等で同じMakefileを呼べる。

`http://127.0.0.1:5173/me` を開く（従来の `/` も同じ保護画面へ移る）。未認証なら `/login?return_to=%2Fme` でログインし、本人情報取得の成功後に `/me` へ戻る。開発サーバーは127.0.0.1で待ち受け、5173が使用中なら別ポートへ移らず終了する。終了はCtrl+C。

送信を確認するには、別の端末で[Backend](../backend/README.md)をルートの `make backend-dev` で起動する。既定のAPP_ORIGINは `http://127.0.0.1:5173`。Viteが `/api/` を `http://127.0.0.1:8080` へ中継し、Origin・Hostを書き換えない。ブラウザからGoのポートへ直接送信しない。

短い仮パスワードなら入力不正、未知ID・誤パスワードなら認証失敗となる。作成済みの架空Userで成功すると、本人情報APIから取得したログインIDを表示する。本人情報APIの401ではフォーム、通信・内部障害ではエラーと再試行ボタンを表示し、認証状態を断定しない。ログイン成功後は入力パスワードを画面メモリから消す。Cookieはブラウザが扱い、Vueから値を読まない。実際に使うパスワードを検証に使わない。Goが停止している場合はViteが中継エラーのHTTP500を返すことがあり、その場合は内部障害表示となる。ブラウザ自体のfetchが失敗した場合は通信失敗表示となる。

送信中は入力欄とボタンを無効化して状況を表示し、重複送信・自動再試行を行わない。入力不正の案内とCSRF拒否の文言は検証用の仮表示で、未合意の最終画面仕様として固定しない。

```bash
make build
```

Vueの型検査後に `dist/` へ生成する。TypeScriptはVue型検査ツールの対応範囲に合わせ6系に固定する。PrimeVue 4とテーマ1系はMIT版を固定し、4系の保守はセキュリティ修正のみであることに留意する。依存更新時はライセンス・互換性・監査結果を再確認する。`public/THIRD-PARTY-NOTICES.txt` は配布するライブラリの著作権・ライセンス表示であり、依存更新時に対応する表示も更新する。

旧AI案の `src/app.js`・`src/client.mjs`・`src/client.test.mjs`・`src/style.css` と `scripts/` は参考として保持しており、新しい画面からは参照しない。`make test-legacy` は旧案の通信処理試験であり、新画面の認証試験ではない。

## 保護URLとログイン復帰（Issue #55）

`/me` は保護画面、`/login` はログイン画面。本人情報APIの401を確認してからログインへ移り、成功後は `return_to` の許可済み保護パスへ戻る。外部・未知・不正・重複の戻り先は採用せず、`/me` を既定とする。現在は `/me`・`/collections`・`/reports` と設定・ReportのULID詳細パスを許可し、任意クエリ・フラグメントは扱わない。[画面仕様](../docs/画面仕様/保護URLとログイン復帰.md)と対応する。

ルートの許可一覧と純粋な検証関数は `src/navigation.ts`。後続画面を追加するときだけ、その画面・所有者確認・試験と同時に一覧を拡張する。Vue RouterのWeb Historyを使い、認証遷移はrouter.replaceで現在の履歴を置換する。再読込時のHTML配信はViteのSPA fallbackを使い、本番nginxの設定は #21で検証する。

```bash
make test
```

Vitestで、既知パス・外部URL・符号化・逆スラッシュ・重複・login/APIへの戻り先に加え、Vue Test Utilsと実Routerのmemory historyで画面操作・API失敗・遅い応答・ログアウトを確認する。ルートでは `make frontend-test`、`make check` でも実行する。ブラウザではCookieのない新しいプロファイルで `/me` を直接開き、ログインURL・認証成功後の `/me`・再読込を確認する。500や通信障害では未認証と断定せず再試行できること、不正な `return_to` で外部へ移らないこと、ログアウト失敗後に履歴移動しても本人情報が戻らないことも確認する。秘密値をURLや公開記録へ書かない。

## ログアウト（Issue #54）

ログイン後の本人情報画面で「ログアウト」ボタンを押す。本人情報・入力値を消去して処理中を表示し、APIの204を確認したら空のログインフォームへ戻る。失敗時は個人データを戻さず、完了を断定しない説明と再試行ボタンを表示する。再試行で本人情報APIを呼び直さない。画面仕様・API契約は[ログアウト仕様](../docs/画面仕様/ログイン・本人情報表示.md#ログアウトissue-54)を参照する。

ローカルDBには事前に `make db-migrate`（リポジトリルートまたはBackend）で版2を適用する。DB・API・Frontendを既存手順で起動したら、架空Userで次を確認する。

1. ログインして本人情報を確認し、Tabでログアウトボタンへ移動し、Enterで実行する。
2. ログインフォームへ戻り、入力欄が空で本人情報が残らないことを確認する。画面を再読み込みしても未認証であることを確認する。
3. もう一度ログインし、ブラウザの開発者ツールで通信をオフラインにしてからログアウトする。本人情報が消え、完了未確認と再試行が表示されることを確認する。
4. オンラインに戻して再試行し、ログインフォームへ戻ることを確認する。処理中の重複送信や本人情報の自動再取得をしない。

オフライン操作はブラウザの模擬通信障害であり、実DB障害の確認とは区別する。実DBのタイムアウト・旧トークン再利用・他セッション維持はBackendの `make verify-logout-api` で検証する。検証パスワード・Cookie・Networkの秘密値をスクリーンショットや公開記録に残さない。実行した試験と未確認範囲はIssueへ記録する。

## Collection設定の確認（Issue #56）

Backend側でmigrationと `make seed-collections LOGIN_ID=fictional_user` を実行した後、`/collections` を開く。未認証ならログインへ移り、成功後に同じパスへ戻る。一覧から自分の設定詳細を開き、調査方針・Schedule・対象・情報源を読み取り専用で確認する。

0件・Scheduleなし・対象/情報源なし、404、取得失敗と「設定を再試行」を区別する。別Userの詳細IDを直接開いても「見つかりません。」とし、情報を表示しない。ログアウト開始時に設定を即座に消し、遅い設定応答や履歴操作で復活しないことも確認する。URL・本文をHTMLとして挿入せず、外部取得や編集操作は行わない。

保存済みReportは `/reports` の最小一覧と `/reports/{ULID}` の詳細で取得する。本文は限定Markdown変換/sanitize後に表示し、未対応時は安全なテキストへ戻す。再試行・401/404・ログアウト中の消去は[画面仕様](../docs/画面仕様/保存済みReportの取得.md)に従う。

## Frontendの責務とテスト（Issue #68）

`src/App.vue` はRouterに接続してsessionを一度だけ生成・provideし、`layouts/AppLayout.vue` を組み立てる。各ページ・featureはinjectで同じsessionを参照する。認証情報を別々に保持するstoreや永続ストレージは追加しない。

| 配置 | 責務 |
|---|---|
| `router.ts` / `navigation.ts` | ページと履歴の管理 / 安全な保護パスと戻り先の許可一覧 |
| `layouts/` / `pages/` | 共通枠・認証状態・フォーカス / URL単位の表示の接続 |
| `features/auth/` | ログインフォーム、共有session、認証APIと応答検証 |
| `features/collections/` / `features/reports/` | 各表示、取得接続、API、型と応答guard |
| `composables/useResource.ts` | 読み込み・再試行・timeout・消去と遅延応答破棄 |
| `components/` / `api.ts` | 共通のエラー表示 / 同一Origin・no-store通信とJSON読取り |

認証の変更はsessionの操作に集約する。resource側は401 unauthenticatedを確認した場合だけ、世代付きのexpire通知をsessionへ送る。route変更・ログアウト・unmountでは表示を消し、前の世代の応答を採用しない。内部redirectだけを識別し、同時に起きた利用者の遷移も処理する。ログアウト失敗中は履歴・BFCacheによる再認証を止め、ログアウトの再試行だけを許可する。

```bash
make test
make coverage
```

ルートの `make frontend-test` / `make frontend-coverage` も同じ入口。通常の `make check` に画面テストを含む。`coverage/` にHTMLとJSON summaryを出力し、Git対象外とする。V8 coverageのincludeは `src/**/*.{ts,vue}` で、入口・画面・composable・API・guardの現用コード全体を分母に含む。旧不採用のJavaScript案は現entryから参照されず対象外で、`test:legacy` と区別する。coverageはDOM環境内の実行率であり、実ブラウザ・Cookie・DB・HTTPサーバーを接続した確認の代用にはしない。coverage向上だけを目的としたsnapshotや内部関数の呼出しでなく、正常・失敗・競合時の表示と操作を確かめる。

[Vueのテスト指針](https://vuejs.org/guide/scaling-up/testing.html)に沿ってVitestとVue Test Utilsを採用し、[実Routerの試験](https://test-utils.vuejs.org/guide/advanced/vue-router)と[V8 coverage](https://vitest.dev/guide/coverage.html)を使用する。Node 26・Vue 3.5・Vite 8とのpeer互換を確認してexact pinする。Router 5.4.0、Vitest/coverage-v8 5.1.0、jsdom 31.0.0、Vue Test Utils 2.5.1を採用。これらはMITで、テスト依存は配布bundleへ含めない。インストール時のscriptsは引き続き無効にする。

テスト用間接依存の監査には既知のmoderate指摘が残る。`js-beautify` のCLIが利用するglob/editorconfig/minimatchに、[AbortSignal後の探索](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4)と[globpartsによる負荷増大](https://github.com/advisories/GHSA-g3hv-75wg-m249)がある。現テストは固定の架空データを使い、このCLIや任意ファイル探索を呼び出さない。これらのテスト依存はアプリからimportせず、本番bundleへ含めない。上流の依存制約内では解消できず、動作互換を保証できないmajor overrideは加えない。依存更新時に開発用も含めて再監査する。本番依存の監査と開発依存の指摘は区別して記録する。

## Collection本体の新規作成（Issue #18）

`/collections` の「新規作成」から `/collections/new` を開く。名前・任意の調査方針・初回期間を入力し、「保存」で停止中の本体を作る。保存後は既存詳細へ移り、一覧や再読み込みでも取得できる。対象・Schedule等の編集は後続範囲。

新規ページは `pages/CollectionCreatePage.vue`、下書き・保存・要求番号・再試行・遅延応答破棄は `features/collections/useCollectionCreation.ts` が担当し、既存の単一sessionを共有する。結果不明時は同じ要求番号・同じ本文を再送するため、入力を保持して編集を止める。画面離脱・再読み込み後は一覧で確認し、下書きや番号を永続保存しない。[画面仕様](../docs/画面仕様/コレクションの新規作成.md)と[API](../docs/API仕様/post-api-collections.md)を参照する。

`make test` は入力、保存成功、初回期間、400/401/403/409、500/不正成功/通信失敗後の同一要求再送、二重送信、ログアウトと遅延応答を確認する。実ブラウザ・実DBはルートの `make verify-collection-creation` で別に確認する。配下の編集やMarkdown表示まで完成したことを意味しない。

## Collection本体の編集（Issue #74）

停止中Collectionの詳細URLで「編集」を押す。取得済みの本体3項目を変更し、保存後は詳細を再取得する。競合時は下書きを保持し、「最新値を確認」→比較→「最新値で編集し直す」で最新値へ入力を置き換え、必要な変更を再入力する。結果不明では編集を止め、同じ本文・要求番号で手動再試行する。離脱・ログアウト等で破棄後は詳細で現在値を確認する。

`CollectionFields.vue` と `form.ts` は作成/編集の入力部品・検証を共有し、通信と状態処理は各composableが担う。版と再試行の記録はAPI/DBが担当する。詳細は[編集画面](../docs/画面仕様/コレクション本体の編集.md)と[更新API](../docs/API仕様/patch-api-collections.md)。版10を明示適用したAPIが必要で、画面URLはAPIのAPP_ORIGINと一致させる。既定は `http://127.0.0.1:5173`。

`make frontend-test` / `make check` のVitestと、ルートの `make verify-collection-update` の隔離実DB/Chromiumを使う。前者の応答注入と後者の実保存・実DB停止を区別し、実行結果は#74/PRへ記録する。未保存変更確認は#42、配下CRUD等は#18の残りとして維持する。

## Collectionと対象の一括編集（Issue #78）

停止中Collection詳細の「編集」で、本体の下にある対象名・任意の補足指示を編集する。「対象を追加」は未保存の行を追加し、「追加を取り消す」はその行だけを破棄する。最後の「保存」で本体と変更対象を一緒に保存する。対象変更は1回20行以内、登録総数の上限ではない。既存対象の状態・順序・情報源は維持する。新規対象のONではCollectionの生成を開始しない。

競合時は対象を含む下書きを残して最新値を比較し、明示操作で編集し直す。結果不明中は入力を止めて同じ要求を再送する。離脱・認証変化では下書きを消去する。make frontend-testは応答注入による試験、make verify-collection-targetsは隔離DB・Chromiumの通し試験であり、確認範囲を区別する。

### 情報源の一括保存（Issue #80）

停止中Collection詳細の編集で、保存済み対象の情報源を追加・編集し、本体/対象とまとめて保存する。新規対象には保存後に情報源を追加する。URLは検証・保存・テキスト表示だけを行い、外部アクセスや正規化・重複検査・削除は行わない。契約は[更新API](../docs/API仕様/patch-api-collections.md)を参照。

検証入口はルートの `make verify-collection-sources`。使い捨てCompose project/架空データでmigration 9→10→11→12と再適用、旧要求digest/再送、原子的保存・所属/CSRF・URL/上限・競合/rollback・履歴不変、Chromiumを検証する。goose版12は情報源の必要列権限のみ追加し、通常DB/本番/EC2には適用しない。通常起動/配備への自動migrationも追加しない。

## Scheduleの一括編集（Issue #82）

停止中Collectionの既存編集モードに `ScheduleFields.vue` を表示し、日本時間のraw cronを本体・対象・情報源と一括保存する。入力例・構文エラー・入力保持を表示し、未設定なら空欄で保存できる。既存式の空欄削除は許可しない。`scheduleForm.ts` とGoのvalidatorは共通fixtureで受理構文を検証する。限定構文・#19との共通契約は[更新API](../docs/API仕様/patch-api-collections.md#schedule追加編集)、操作は[編集画面](../docs/画面仕様/コレクション本体の編集.md#schedule追加編集issue-82)を参照。

既存の版・競合比較・明示再編集・結果不明時の本文/番号保持・認証消去・遅延応答破棄を拡張する。単体は `make frontend-test`、実DB/Chromiumはルートの `make verify-collection-schedules`。API側の版13権限が必要だが、通常起動にmigrationを追加しない。次回予定表示・生成処理は含めない。

## 保存済みReportのMarkdown表示（Issue #84）

`ReportMarkdown.vue` が保存本文だけを受け取り、`markdown.ts` の最終変換/sanitize結果だけを局所的に挿入する。未対応/例外時は固定案内とVue補間の本文テキストへ閉じる。本文props由来の計算だけを持ち、独立キャッシュやHTMLの永続保存はしない。useResourceの取得開始・route/認証変化・logout消去と遅延応答破棄をそのまま使う。許可タグ/タグ別属性・URL条件は[画面仕様](../docs/画面仕様/保存済みReportの取得.md#安全なmarkdown表示)を正とする。

markdown-it 15.0.3（MIT）とDOMPurify 3.5.0（Apache-2.0選択）をexact pinする。markdown-it v15の型・ESM/browser export同梱を確認し、@types/markdown-itは不要。固定runtime Node26.13.0・Vite8・TS6で型/ビルドを確認する。DOMPurify3.5.0のsecurity fixesを含むreleaseと公式README、安全性/移行資料を確認した。Marked18.3.0も代替だが、HTML/画像/拡張構文の制御を明示できる点でmarkdown-itを採用した代理判断。独自regexのみのsanitizeやhighlight依存は追加しない。

追加のruntime依存は既存lock項目を変えず追加分だけ固定する。インストールは既存 `make frontend-install`（npm ci --ignore-scripts）を使う。`public/THIRD-PARTY-NOTICES.txt` へ追加依存/同梱helperのcopyrightとlicenseを反映し、Viteが配布dist直下へ複製することを確認する。DOMPurifyはパッケージLICENSEのApache-2.0を選択し、配布bundleのcopyrightと同梱MIT regenerator helperの表示も保持する。今回追加パッケージに別のNOTICEファイルはない。ライブラリ本体のソース変更はしない。CLI依存argparseのPSF表示も含める。依存更新時は現物license/NOTICEと監査を再確認する。

`make frontend-test` はXSS/URL回避/タグ別属性/HTML文字表示/画像無要素、未対応/変換/sanitize例外、props変更と実Router/認証・遅延応答消去を確認する。実DB・Chromiumはルートの `make verify-report-markdown`。Go/Node・Docker/Compose・既存goose・Chromiumが必要で、不足ライブラリは既存BROWSER_LIBRARY_PATHを使いOSへ導入しない。専用project/空きport・架空User/別namespaceの保存Reportで、深いURLからの復帰/再読込/表示/消去、ネットワーク要求監視、本文/API/履歴不変、通常DBの稼働状態不変と専用環境除去を検証する。外部要求はCDPで送信前に拒否するガードを置き、試験の成功条件は拒否0件・外部要求の発生自体0件とする。リンク先はクリックしない。

架空データを追加する手動入口は `make seed-markdown-reports LOGIN_ID=fictional_user`。既存の架空UserとDB準備が必要で、別namespaceへ原子的追加する。既存v3履歴は変更せず、再投入時の差異を拒否する。seedは生成成功の実証ではない。通常DBへの実行は今回の作業では行わず、使い捨てrunner内だけで検証する。

実施結果・監査結果・未確認のブラウザ/配備等はIssue/PRへ記録する。API/DB/schema/生成処理は不変、migration不要。本格一覧/home・Collection有効化・#19/#20/#21は対象外。

## Report一覧の追加取得（Issue #86）

`useReports.ts` は既存useResourceによる初回取得/消去を共有し、Report専用の追加項目/next cursor/実行中/エラーだけを保持する。共通useResourceは変更しない。初回は30件まで、「もっと読む」で続きを取得し、失敗/timeout/不正payloadでは既存項目と同じcursorを保持する。二重取得を抑止し、認証/route/初回データ置換・logoutで続きの状態も破棄する。401は正しい `/reports` へログイン復帰する。詳細のMarkdown表示は維持する。

cursorはサーバー値のまま再送し、解読/再生成/URL・storage永続化しない。順序検査はUTC小数部6桁の文字列とULIDで行い、JSミリ秒へ丸めない。重複/逆順/境界重複・繰返しcursor等は全ページを追加せず、再試行できる。日時のDate利用は暦の妥当性だけで順序比較には使わない。全ページsnapshotではなく、新しい先頭は再読込で確認する。[API](../docs/API仕様/get-api-reports.md#30件ずつのcursor追加取得)・[画面](../docs/画面仕様/保存済みReportの取得.md#保存済み成功report一覧の追加取得)を参照。

単体/実Routerは `make frontend-test`、使い捨てDB/Chromiumはルート `make verify-report-pages`。0/30/31/35・同一秒マイクロ差とID逆順・同日時、追加取得失敗/timeout/重複・不正payload・二重操作、認証失効/離脱/遅い応答を確認する。npm ci --ignore-scripts・既存runtime/依存/lockは維持し、追加依存はない。

## 日付別の保存済み予定・結果（Issue #88）

`/reports` 内のリンクから `/reports/daily` を開き、日本時間の日付ごとに保存済みJobの4状態を30件ずつ読む。成功だけ既存Report詳細へ移動する。手動更新は先頭30件へ戻る。生成処理は実行しない。

検証入口はルートの `make verify-daily-jobs`。使い捨てDB/架空データ/Chromiumによる範囲と事前条件は[Backend手順](../backend/README.md#日付別の保存済み予定結果issue-88)を参照する。

契約は[Job一覧API](../docs/API仕様/get-api-generation-jobs.md)と[日付別画面](../docs/画面仕様/日付別の予定・結果一覧.md)。検証結果はIssue/PRに記録する。

## Collection別の保存済み予定・結果（Issue #90）

既存のReport一覧から `/reports/collections` を開き、本人の現在Collectionを選ぶと `/reports/collections/{ULID}` で履歴を読みます。成功行だけ保存済みReport詳細へ移動し、4状態/予定時刻は日本語/JSTで表示します。collection設定画面からはReportへの導線を追加しません。

候補一覧と選択履歴は直列に取得し、同じuseResource/AbortSignalと認証epochに従います。選択・route・認証・更新ごとに古い取得/ページ/cursorを破棄し、遅い前Collection応答を混ぜません。続き失敗時は既表示とcursorを保持して同じcursorを再試行します。APIの所有権確認が最終境界であり、削除/他者/不存在を同じ404にします。削除済み履歴は日付別画面に残ります。

`make frontend-test` の応答注入試験とルート `make verify-collection-jobs` の実DB/Chromium試験を区別します。画面/API契約は[Collection別画面](../docs/画面仕様/Collection別の予定・結果一覧.md)と[Job API](../docs/API仕様/get-api-collection-generation-jobs.md)を参照してください。
