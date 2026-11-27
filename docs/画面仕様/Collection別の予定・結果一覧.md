# Collection別の予定・結果一覧（Issue #90）

既存 `/reports` 画面の「Collection別の予定・結果」リンクから保護URL `/reports/collections` を開く。本人の現在Collection一覧を `GET /api/collections` で取得し、1つ選択すると `/reports/collections/{正規ULID}` へ移動する。選択中も同じCollection選択を操作できる。主要ナビは増やさず、Collection設定一覧・編集からはReportへの導線を作らない。

## 表示・操作

選択画面では現在Collectionだけを名前とともに選べる。選択Collectionの保存済みJobを[Collection別Job API](../API仕様/get-api-collection-generation-jobs.md)から予定時刻の新しい順に最大30件取得する。各行は明示的なAsia/Tokyoで予定日時を表示し、状態を待機中・実行中・成功・失敗と日本語表示する。成功だけ既存 `/reports/{report_id}` にリンクし、他状態はリンクなし。Collection選択候補が空なら、Collectionがないことを案内する。存在するCollectionにJobがない場合は「まだレポートはありません」と表示する。

「更新」は既知候補を直ちに消去して候補一覧を先に取り直し、選択Collectionが現在も候補にある場合だけ履歴の先頭30件を読み直す。候補GETが成功したら、履歴取得の完了を待たず検証済みの現在候補へ置き換える。候補GETまたは履歴GETが失敗した場合は候補も消去する。削除済みCollectionの404では最新候補だけを表示し、削除されたIDを候補へ戻さない。画面を開き直す/再読込も先頭から取得する。続きは既存Job APIのcursorで30件ずつ読み、全ページsnapshotは保証しない。続き失敗時は既表示行とcursorを保持し、同じ続きから手動再試行する。

Collection切替時はURLを切り替え、前の一覧・続き・cursorを直ちに消去する。候補取得→履歴取得は直列で、同じAbortSignal/画面世代の配下とする。候補取得中に表示できる候補は、同じ認証epoch内で直前にサーバー確認した一覧に限る。新しい候補応答を検証した時点で即置換し、AbortSignalが失効した遅延応答は反映しない。明示更新開始、候補/履歴取得失敗、認証epoch変更/logout、Collection画面外へのroute遷移では候補を含む取得状態を消去する。削除済みCollectionの404応答では、当該IDを除いた最新候補を表示する。多重の「もっと読む」は抑止する。異常200、重複行、逆順、cursorの再発行/循環を拒否し、既表示を保つ。

初回候補取得/履歴取得中・失敗・再試行・空・404を区別し、内部エラーや技術的failure理由は表示しない。Collectionが削除済み/不存在/他User所有の場合は共通の「見つかりません」とし、削除済みCollection専用の履歴導線を作らない。削除後も残る履歴は日付別一覧で読む。

## URLと認証

`/reports/collections` と正規ULIDを持つ `/reports/collections/{ULID}` は保護URL allowlistへ登録する。未認証で直接開いたURLはログイン成功後に同じパスへ復帰し、任意query/fragmentは保持しない。Collection IDはURLだけに置き、cursor・認証情報・個人データをURL/localStorage/sessionStorageへ保存しない。

## 確認観点

- 既存`/reports`から選択へ進み、本人Collectionを選択→4状態/成功詳細→戻る/再読込→logoutを確認する。
- Collection 0件、Job 0/30/31/35件、複数Collectionの混合Job、UTC/JST日付境界・microsecondを確認する。
- 深URLのログイン復帰、本人/他者/不存在/削除済みの404と、削除後も日付別で読める履歴を確認する。
- 選択高速切替・旧応答遅延・更新・多重操作・失敗保持/再試行・不正200/順序/cursor・401/route/logoutの消去を確認する。
- 設定/Job/Attempt/Report履歴全体が閲覧前後で不変であり、成功以外にリンクがないことを確認する。
