# 保存済みReportの取得

Issue #57の委任に基づく最小読み取り専用画面。`/reports` に本人のReportへのリンクを置き、`/reports/{ULID}` にJob時点のCollection名、更新確認期間、生成完了日時、保存済み本文を表示する。日付別/Collection別の本格一覧や編集は #18へ残す。保存本文の安全なMarkdown表示は後段の現行契約に従う。

本人情報・設定のナビゲーションから開ける。保護URLのallowlistへ正規ULIDの詳細と一覧を追加する。未認証で詳細を開くと `/login?return_to=...` へ移り、認証成功後に同じReportを取得する。戻り先は実装済みの内部パスのみ許可し、外部URL・query/fragment・不正IDは許可しない。

| 状態 | 表示・操作 |
|---|---|
| 読込中 | Reportを読み込んでいます。古いデータは消去 |
| 空 | 保存済みReportはありません。 |
| 成功 | 一覧リンク、または保存済み本文。全成功JSONの型検証後だけ表示 |
| 404 | 見つかりません。他者と不存在を区別しない |
| 500/不正200/通信障害/15秒timeout | 固定エラーと「Reportを再試行」。URLを維持 |
| 401 | 個人データを消去し、そのURLを安全な戻り先としてログインへ |

本文は[安全なMarkdown表示](#安全なmarkdown表示)の限定rendererを使う。生HTMLは文字として残し、安全なリンク以外はクリック可能にしない。旧#57のpre表示から表示方法だけを変更し、保存本文/APIは変更しない。整形表示の未対応/例外時だけ安全なpreテキストへ戻す。

ログアウト開始、画面移動、再取得開始で保持中のReport一覧/本文を消去する。ログアウト失敗中も表示し直さず、古い世代の遅延応答や履歴操作で復活させない。API契約は[保存済みReport取得](../API仕様/get-api-reports.md)、共通認証導線は[保護URLとログイン復帰](保護URLとログイン復帰.md)を参照する。

## 安全なMarkdown表示

保存済みbody_markdownの表示だけを拡張する。AI断片の許可構文・生成時検証と区別する。局所コンポーネントが本文propsから変換/sanitizeした結果だけを挿入し、呼出側が任意HTMLを渡す入口を作らない。sanitize後の文字列追記・DOM加工をしない。

markdown-it 15.0.3（MIT）とブラウザDOMPurify 3.5.0（Apache-2.0を選択）を固定採用する代理判断。markdown-itの型/ESMは同梱、@types/markdown-itは追加しない。Node26.13/Vite8/TS6の実互換は型検査/ビルドで確認する。Markedも候補だがHTML/画像/構文の制御を明示できる点を優先した。新しいhighlight/外部資源依存は追加しない。

html:false・linkify:false・typographer:false。許可タグはh1〜h6,p,strong,em,ul,ol,li,pre,code,a,br,hr。表/引用等の拡張は追加しない。inline/fenced/indented codeはテキストとして表示し、言語class/highlightなし。HTMLは文字として表示し、画像はalt文字だけにする。許可属性はaのhref/rel/referrerpolicyとolのstartだけ。id/name/style/イベント/data/aria/target/ping/src/srcsetや埋込・画像・SVG/MathML等の実要素は作らない。DOMPurifyの限定allowlistとリンク検証を併用する。

URLはMarkdownライブラリの検証とブラウザURL parserの両方を通し、絶対http/https・ホストあり・username/passwordなしとする。制御文字/空白/バックスラッシュ、相対/プロトコル相対/fragment-only、他schemeはクリック可能にしない。HTMLエンティティ/Markdownエスケープを解釈したURLも再検査する。空userinfoの@もraw authorityで拒否し、percent decode後の制御文字/バックスラッシュも拒否する。percent escapeの不正・UTF-8不正は解釈差を避けるためリンク化しない。パスの正当な%20は許可し、全URLの正規化や再帰decodeはしない。拒否時は内容を文字として残す。情報源入力の2,048文字上限は転用しない。リンクはtargetなしの同一タブ、rel="noopener noreferrer"・referrerpolicy="no-referrer"。修飾キーによるブラウザ標準操作は妨げず、自動fetch/prefetch/preloadは行わない。クリック先の安全性や到達性・#20のSSRF対策を保証するものではない。

DOMPurify未対応・Markdown変換/sanitize例外時は固定案内とVue補間による保存本文のテキスト表示にする。未処理HTMLへfallbackしない。生成HTMLを永続化・キャッシュせず、useResourceの取得開始/認証/route/logout消去と遅延応答破棄を引き継ぐ。確認は限定rendererの単体、実Router/認証回帰、専用Compose/架空ReportとChromiumのネットワーク監視で行う。配布のlicense/NOTICEは[Frontend手順](../../frontend/README.md)と既存THIRD-PARTY-NOTICESへ反映する。

一次資料：[markdown-it安全性](https://github.com/markdown-it/markdown-it/blob/15.0.3/docs/safety.md)、[v15移行](https://github.com/markdown-it/markdown-it/blob/15.0.3/docs/migration/migration_v15.md)、[DOMPurify](https://github.com/cure53/DOMPurify/tree/3.5.0)、[3.5.0 release](https://github.com/cure53/DOMPurify/releases/tag/3.5.0)。

## 保存済み成功Report一覧の追加取得

/reports初回は30件まで。「もっと読む」は続き取得中無効、成功時だけ末尾へ追加し、next_cursor:nullなら終了を表示する。失敗/timeout/不正200/重複/逆順/不整合payloadでは既存項目と同じcursorを保持して手動再試行する。初回の読込/エラー/0件は既存useResourceに従う。認証/route/logoutで一覧/本文/追加状態/cursorを消し、遅い応答を破棄する。cursorはURL/storageへ保存せずそのまま再送し、離脱/再読込は初期30件。マイクロ秒を保持する時刻文字列比較で順序検査し、JS Dateのミリ秒丸めに依存しない。

具体契約は[取得API](../API仕様/get-api-reports.md#30件ずつのcursor追加取得)を参照する。
