# 管理コマンドによるパスワード再設定（Issue #58）

対象Userのパスワードハッシュ更新と、そのUserの認証セッション全件DELETEを同じDBトランザクションで確定する。Web画面・メール再設定・端末管理・時間失効は追加しない。[入力条件と共通Argon2id](password-hashing.md)はUser作成・ログインと共通である。

## 操作と失敗時の扱い

[初期設定・既存DBの追加設定](local-postgresql.md#再設定用の専用ロール追加issue-58)とmigration後、ルートまたはBackendで実行する。

```bash
make password-reset LOGIN_ID=fictional_user
```

管理者は新しいパスワードを端末で2回、非表示入力する。Linuxの既存TTY入力を再利用し、Ctrl+C・終了時にechoを復元する。引数・Make変数・パイプでアプリのパスワードを渡さない。IDは既存のASCII正規化、パスワードはNFC後15〜128コードポイント・確認一致を検証する。新しいsaltでArgon2idを計算してからDBへ接続する。同じパスワードへの再設定でもPHCは変わり、既存セッションを失効する。

成功表示はcommit後だけ。対象なし・入力不正・不一致は再設定しない。DB処理中の失敗は成功と断定しない。特にcommit時の通信断では結果不明の場合があり、DBの状態を確認してから手動で再実行する。再実行すると、前の再設定後に新しく作成されたセッションも失効する。自動再試行は行わない。端末入力後のハッシュ・DB処理に15秒timeoutを適用し、ロック待機もキャンセルする。これはセッションの時間失効ではない。

## ログインとの競合制御

PostgreSQLのtransaction advisory lockをUserごとに共通使用する。キーは `hashtextextended('watchbrief.credentials:' || user_id, 0)`。衝突した場合は余分に直列化されるだけで、対象Userの条件は各SQLに残る。transaction終了時に解放されるロックを使い、接続プールへsession lockを残さない。

- ログインはUser読取・パスワード照合・トークン生成をtransaction外で済ませる。Read Committedのtransactionを開始し、共通ロックを取得した**後の別SQL**で現在のPHCを再読取する。照合したPHCと完全一致する場合だけsessionをINSERTしてcommitする。変わっていたら通常の認証拒否（401）、DBエラーなら内部障害（500）とし、Cookieは発行しない。
- 再設定は対象IDを取得し、Read Committedのtransactionで同じロックを取得してからhash・updated_atをUPDATEし、対象Userのsession全件をDELETEしてcommitする。途中エラーは両方rollbackする。updated_atは更新文の実行時刻を `clock_timestamp()` で記録する。
- ログインが先にロックを取れば、そのsessionは後続の再設定で削除される。再設定が先なら、古いPHCで照合済みのログインもロック待機後の再読取で拒否される。再設定同士も直列化され、最後にcommitしたhashだけが残る。

人の入力待ちやArgon2id処理中はDBロックを保持しない。PHC照合後の再読取はRead Committedの各SQLのsnapshot更新に依存する。Repeatable Readへ単純に変更しない。APIへusers UPDATE権限を渡す必要のある `SELECT FOR UPDATE/SHARE` は使わない。根拠は[SELECTの必要権限](https://www.postgresql.org/docs/current/sql-select.html)、[advisory lock](https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS)、[Read Committed](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-READ-COMMITTED)。

認証済みで処理中のAPI要求は再設定後も完了する場合がある。再設定は既に返したデータや処理中の要求を取り消さない。次の認証チェックでは旧トークンを拒否する。ログインのcommit直後に再設定がcommitした場合、ログインが204を返してもそのCookieは既に失効している場合がある。画面は従来どおり本人情報取得に成功するまでログイン完了表示を確定しない。

## 最小DB権限と索引

専用 `watchbrief_password_resetter` にDB CONNECT・app USAGE、usersのid/login_id SELECT、password_hash/updated_at UPDATE、auth_sessionsのuser_id SELECT・DELETEだけを付与する。旧hash・token_hash・設定・Reportは読めず、Userの作成/削除、login_id変更、session作成、DDL、ロール管理はできない。これらはカラム/テーブル単位の権限であり、対象Userの行制限は管理コマンドの固定SQLが担う。API・作成者・migrationの既存権限は増やさない。

全件削除は `WHERE user_id = 対象ID`。版7で専用ロールへ権限を付与し、版8で `auth_sessions(user_id)` の索引を追加する。通常API起動にprovisioning・migrationを含めない。新規ロール作成にはpostgresを使い、CREATEROLEを持たないgooseロールとは分ける。秘密ファイルの扱い・既存環境手順は[ローカルPostgreSQL](local-postgresql.md#再設定用の専用ロール追加issue-58)を参照する。

## 検証の入口と範囲

```bash
make verify-password-reset
make db-verify
```

前者は専用 `.local/password-reset-verification.json`（600）の架空Userを用意し、既存のUser作成・ログアウトfixtureのパスワードを変更しない。専用Userには架空設定と保存Reportもseedする。PTY非表示・入力失敗・Ctrl+C復旧、2旧トークン401、旧パスワード401、新パスワード成功、同パスワード再設定の失効を実API・実DBで確認する。全他User・他session・全業務行のsnapshot不変と秘密値のAPIログ非混入も照合する。普段のAPIとDBは停止しない。専用User・架空業務データと非公開資格情報は保持し、この試験が発行したsessionは最後に失効する。

続くGo integration試験は独立した専用Userと実Argon2id・ログインユースケースを使い、別接続のpg_locksで未取得advisory lockを観測してから先行transactionを進める。両順序と再設定同士、勝ったパスワードの照合、UPDATE後の実DELETE失敗による全体rollback、待機timeoutを検証する。試験の失敗注入は該当接続のSQLだけを差し替え、既存ロール権限・triggerを変更しない。各実行は専用架空User1件を保持する。大量の繰返しは避ける。

`make db-verify` は許可/拒否を実SQLで確認し、fixtureをrollbackする。AWS・本番TLS・実管理接続・commit応答喪失・OS強制終了・全APIの進行中要求を止める試験は対象外。実行結果やレビュー結果はIssue/PRに記録し、この文書を実施履歴にしない。
