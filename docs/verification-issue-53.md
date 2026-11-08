# Issue #53の検証記録・引き継ぎ

> 旧案のGoコード（auth/config/httpapi/store）は、内容を変えず[参考ディレクトリ](../reference/issue-53-previous-proposal/README.md)へ隔離した。以下のコード配置・実行例は当時の記録であり、現在の起動手順として実行しない。

> **Issue #53の既存案について**：以前のAIが作成した未完成の実装案と、それに伴う文書変更を参考として保持している。具体バージョン、ハッシュ、Cookie・CSRF、migration等は、今回利用者と確認したもの以外は採用済みと扱わない。過去の検証結果も今回の実動作確認とは区別する。現在の進め方は[作業計画](work-plan.md#利用者主体で子issueを進める運用合意済み)を参照する。

2026-10-31 JST時点の担当セッション記録。利用者の「docsへ確認状況を分けて記録」という指示に基づき、この作業の検証範囲を残す。継続的な仕様の正本とはせず、監督による再実行・レビュー後にIssue/PRへ結果を引き継ぐ。今回はIssueへの投稿・commit・push・PR作成を行っていない。

## 最初の確認

実際の作業先は指定のissue-53 worktree、ブランチ `#53`、開始時の未コミット変更なし。HEADとローカル `#17` は監督の計画コミット `2a99915` に一致した。ほかのworktreeは編集していない。

AGENTS.md、CONTRIBUTING.md、docs/README.mdと全設計文書・作業計画を確認した。GitHub連携で[Issue #53](https://github.com/smofktn/watchbrief/issues/53)・[親 #17](https://github.com/smofktn/watchbrief/issues/17)の最新本文・コメントを取得し、両方open、#53コメントなし、#17に子Issue分割の記録があることを確認した。状態変更・投稿はしていない。シェルのネットワーク失敗とGitHub連携での取得成功は区別する。

## 実際に動作確認済み

- Node 18.19.1/npm 9.2.0で、既存の公開npm cacheのコピーを `/tmp/watchbrief-supervision/issue-53` 内で使い、Vue 3.5.40と23依存packageをoffline取得し `package-lock.json` を生成した。元cacheは書き換えていない。csstypeのcache header形式がnpm9の要求と合わず、一時コピーのAccept metadataを合わせた。package内容・integrityは変更しない。offline installの「0 vulnerabilities」はオンライン脆弱性監査完了を意味しない。
- cacheにあった `node-linux-x64@22.23.3` を同じ `/tmp` 内へ用意し、Node 22.23.3でも `npm ci --offline --ignore-scripts --audit=false`、Frontendのbuildとclientテストを再実行した。buildは固定Vue runtime・画面module・CSS・HTMLをdistへコピーする。
- `node frontend/src/client.test.mjs`：3試験成功。Cookie同一origin送信・JSON/CSRF header・本人情報呼び出し、内部エラー文の非表示・自動retryなし、通信エラー文の固定化を確認した。実API・実ブラウザによる確認ではない。
- JS構文確認、PythonのAST構文確認、非TTYでの接続Secret入力拒否、lockfileのregistry/integrity、文書リンク96件（コード例を除外）と代表的なcredential署名の検査、`git diff --check`。生成dist・node_modules・一時状態がGit候補から除外されることを確認した。
- Frontendのlocalhost起動を試し、`listen EPERM: operation not permitted 127.0.0.1:5173` を確認した。listenerエラーの生stackを出さず、固定JSON分類を出すよう修正し、同じ拒否時の非zero終了を再確認した。起動成功ではない。

再実行可能なFrontendコマンド（通常環境ではPATH上のNode 22.23.3を使う）：

```bash
npm --prefix frontend ci
npm --prefix frontend run build
node frontend/src/client.test.mjs
node --check frontend/src/app.js
node --check frontend/scripts/dev.mjs
node --check frontend/scripts/build.mjs
python3 -c 'import ast,pathlib; [ast.parse(p.read_text(), filename=str(p)) for p in pathlib.Path("scripts").glob("*.py")]'
git diff --check
```

## 設定・文書のみ確認

実装とDDLを読み、Origin完全一致＋専用header＋JSON＋CORS非許可によるログイン前後のCSRF境界、HTTPS/HTTPのCookie分離、Cookieの永続保存・更新、DB側の時間失効なし、SHA-256 token照合、Argon2idの固定パラメーター・ランダムsalt、ULIDの分離を確認した。実行結果から安全性を証明したという意味ではない。

User/AuthSessionの物理型、CHECK・一意制約・FK・索引、所有者／migration／API／User作成ロールとPUBLIC権限剥奪、APIでのDDL不使用、transactionによる版記録とDDLの同時確定をレビューした。

Goテストには正常ログイン・本人情報・新API instanceで同じrepositoryを使う場合、誤パスワード・未知ID・不正JSON・サイズ・CSRF・Cookie属性・重複Cookie・内部障害のsanitizationを用意した。新instance試験を**実プロセス再起動試験の代替**とは扱わない。

実行用の `scripts/verify-login.py` は非表示TTYで実User作成・重複ID・HTTP拒否系・実APIプロセス再起動・同じCookie・ログ非混入を試す。`backend/db/verify.sql` はDB制約と通常権限の拒否をtransaction内で試してrollbackする。手順は[ローカル起動](local-development.md)。

## 未確認・省略とブロッカー

| 項目 | 理由・次に必要な操作 |
|---|---|
| Go dependency取得・go.sum・sqlc生成物・gofmt・build/test/vet/race | Go・sqlcが未導入。シェルのcurlはgo.dev/registry.npmjs.orgのDNS解決失敗。ユーザー領域のGo 1.27.3と依存取得ができる環境で、起動手順のsqlc実行→tidy→gofmt→test/buildを実行し、生成差分をレビューする必要がある。checksum・生成物を手書きしていない。現在はこの理由でGoアプリをそのままビルドできず、完了条件未達 |
| PostgreSQLで新規setup・migration再適用/rollback・制約・ロール認証/拒否 | PostgreSQLの実行ツールが未導入。Docker socket接続は `operation not permitted`。許可されたローカルDB実行環境の用意が必要。OS全体のpackage導入・sandbox迂回はしていない |
| 実User作成・ログイン→Cookie付きAPI→Vue本人情報、誤パスワード等の実API試験、実API再起動後の維持、実ログ非混入 | 上記Go/DBが必要。ユニット試験sourceや設定値から実挙動を断定しない。架空Userはまだ実DBに作成できていない |
| 実ブラウザ・ブラウザ再起動後のCookie維持 | localhost listenerがsandboxでEPERM。ブラウザが開ける許可された環境で再試験が必要 |
| HTTPS・nginx・Secure Cookieの実ブラウザ送信、長期Cookie保存上限、対象ホストでのArgon2負荷 | 今回配備せず #21以降へ引き継ぐ。HTTPの架空データ確認だけでは本番を検証済みにしない |
| 試行制限・認証専用イベント、ログアウト・元URL復帰・設定・Report・再設定 | 指示どおり #49・#54以降の対象として実装・検証を省略した。孤立sessionの削除も未実装 |

## セルフレビューの指摘・修正・再検証・残る制限

全変更対象（既存docsの差分、新規Go/SQL/UI/スクリプト/lockfile/起動手順）を読み直した。仕様の責務・採用時点・対象外機能・Secret非出力・Cookie/CSRF/hash/DB権限・新規setup・試験の意味を確認した。

- 指摘：Userに既存設計の `updated_at` が欠けていた。修正：migrationに追加し、作成時にcreated_atと同じ既定値を設定。再検証：DDL・文書を照合。実DBは未実行。
- 指摘：Go標準HTTP内部ログをそのままJSON loggerへ接続すると、生エラーを含め得る。修正：固定分類だけを出すwriterへ置換。再検証：ログ呼び出し箇所を再確認。Goの実行試験は環境のため未実施。
- 指摘：設定JSONをLimitReaderのDecoderへ直接渡すと、サイズ超過の末尾を読まずにEOFと解釈し得る。修正：8193byteまで読み、8192byte超過を先に拒否。再検証：sourceとサイズ境界のテストを確認。Go実行は未実施。
- 指摘：開発サーバーのlisten失敗で生stackを表示していた。修正：固定JSON分類・非zero終了へ変更。再検証：Node 22.23.3で起動を再試行し、拒否時の出力を確認した。
- 指摘：API起動ログをsocket確保前にreadyとしていた。修正：同期的なnet.Listen成功後にServeとreadyログへ進む。DB接続のtimezone/search_pathも明示した。再検証：sourceを再確認。Go実行は未実施。
- 指摘：bootstrap接続からSET ROLEしても、そのsessionからのownerへのSET ROLE拒否を証明できない。修正：SQLではmembership確認とし、Python検証に実API roleで接続するDDL・role昇格拒否の試験を追加。再検証：Python構文・接続Secretの扱いを確認、実DBは未実行。
- 指摘：Python getpassのTTY不在時fallbackと、HTTP clientの環境proxy利用を許していた。修正：非表示入力不可なら停止し、ローカル検証のproxyを無効にした。再検証：Python構文・非TTY時の停止を確認。
- 指摘：JSON decoderによる不正UTF-8の置換を入力条件の検証前に許していた。修正：4096byte上限の読み取り後にUTF-8を検査し、400を返す試験を追加。再検証：Go sourceを照合、実行は未実施。
- 残る制限：Go・実DB・実ブラウザの検証、Go生成物とchecksum lockfile、gofmtが未実施。監督の独立レビューも未実施であり、Issue #53の実装・検証完了とは報告しない。

制御ログ・cacheは `/tmp/watchbrief-supervision/issue-53` 内に限定した。コミット・push・PR作成・Issue投稿・close/reopen・AWS操作・外部公開は行っていない。
