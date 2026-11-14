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

保存済みReportは `/reports` の最小一覧と `/reports/{ULID}` の詳細で取得する。本文はHTML挿入せずテキスト表示し、再試行・401/404・ログアウト中の消去は[画面仕様](../docs/画面仕様/保存済みReportの取得.md)に従う。

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
