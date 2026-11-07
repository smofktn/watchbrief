# 共通パスワードハッシュ

API・User作成コマンドで共有するArgon2id実装は `backend/internal/security/password` に置く。HTTP・DBの処理は含めず、内側のドメイン入力条件を呼び出す。[参考ディレクトリ](../reference/issue-53-previous-proposal/README.md)の旧AI案 `backend/internal/auth` は今回の共通処理として使わない。

## 初期プロファイル

| 項目 | 設定 |
|---|---|
| ライブラリ | Go公式 `golang.org/x/crypto/argon2`、x/crypto v0.58.0。間接依存x/sys v0.49.0 |
| 方式・版 | Argon2id、version 19 |
| メモリ | 65,536KiB（64MiB） |
| 反復 | 3回 |
| 並列度 | 4 lanes。総メモリを4倍にする設定ではない |
| salt | crypto/randによる毎回新しい16byte |
| ハッシュ | 32byte |
| 保存 | PHC形式の97文字。方式・版・パラメーターと、paddingなし標準Base64のsalt・ハッシュを含む |

RFC 9106の省メモリ向け第二推奨を起点に選び、旧AI案の数値をそのまま採用していない。OWASPのArgon2id最低条件も参照する。パスワードから復号可能な情報を作る方式ではなく、保存値をそのままログインパスワードとして扱うこともしない。

`Hash(ctx, password)` はドメインのNFC正規化後に15〜128コードポイントの条件を確認して生成する。パスワードをtrim・大小文字変換・切捨てしない。`Verify` とUser作成時の確認入力にも同じNFCを適用する。「が」と「か＋結合濁点」は一致するが、全角半角・大文字小文字の違いを同一視するNFKCやcase foldingは行わない。文字数は見た目の字数ではなくNFC後のコードポイント数。生入力は4KiBまでに制限して正規化の処理量を抑え、正規化後に15〜128を検証する。制御文字や弱いパスワードリスト等の追加拒否ルールはまだ未決定・未実装。

`Verify(ctx, password, encoded)` は長さ・方式・版・パラメーター・正規Base64・salt/ハッシュのbyte長を確認してから計算する。現段階で受け付けるのは上記の1プロファイルだけ。保存値に指定された任意の計算量をそのまま実行しない。将来の設定変更では、既存プロファイルの照合と移行を別途設計する。

比較は `crypto/subtle.ConstantTimeCompare` を使う。不一致はfalse、保存形式の不正はErrInvalidHash、入力条件違反はドメインのエラーとして区別する。APIへ接続するときは、内部エラーを認証失敗へ混同せず、エラーへ秘密値を含めない。

## 処理量とキャンセル

`NewHasher()` で作った1つのインスタンスをAPIのリクエスト間で共有する。同時に実行する計算は2件までで、待機中はcontextのキャンセルを受け付ける。Argon2の計算開始後はライブラリの処理自体を途中停止できず、完了後にキャンセルを確認して結果を返さない。インスタンスをリクエストごとに作ると制限が共有されないため、その使い方はしない。

2件実行時のArgon2作業領域は合計約128MiBで、GoのGC保持領域・他の処理等を含むプロセス全体のメモリ上限を保証するものではない。管理コマンドは別プロセスなので同じ枠を共有しない。これはメモリ消費の制御であり、後続Issue #49のログイン試行制限を実装したものではない。

## 検証

Backendディレクトリで実行する。

```bash
make test
make benchmark-password
```

テストではsaltの違い、照合、空白・大小文字の保持、不正な保存形式、入力条件、キャンセルを確認する。`testdata/reference.json` は独立したlibargon2参照実装で作った公開可能な架空値であり、実ユーザーの資格情報ではない。アプリ実装と同じ処理で作った期待値だけに依存しない。

処理時間・検証結果はIssueへ記録する。本番配備先の負荷・同時実行時のメモリは、配備前にその環境で再確認する。部品の試験と[User作成コマンド](../backend/README.md#架空userの作成)の検証、API照合は区別する。APIは同じHasherを全要求で共有し、同時KDF処理を2件に制限する。未知IDでもダミーハッシュを照合し、パスワード不一致と同じ応答にする。これはログイン試行制限（#49）の代わりではない。

NFC処理にはGo公式のx/text v0.43.0を使う。[Go norm](https://pkg.go.dev/golang.org/x/text/unicode/norm)と[NISTのパスワード処理](https://pages.nist.gov/800-63-4/sp800-63b.html#passwordver)を参照する。この一部を参考にしただけでNIST全体に準拠するとは扱わない。

一次情報：[Go argon2](https://pkg.go.dev/golang.org/x/crypto/argon2)、[RFC 9106のパラメーター選択](https://www.rfc-editor.org/rfc/rfc9106.html#section-4)、[OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)。
