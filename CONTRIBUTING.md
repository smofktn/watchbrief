# 開発・運用ルール

WatchBriefは、最初から実装へ入らず、公開境界、失敗時の挙動、運用方法を決めてから小さく作ります。決まっていないことは推測で書き切らず、TODOとして残します。

## 作業単位

- GitHub Issuesを作業と設計判断の記録に使う
- 1つの変更で扱う目的を説明できる大きさに保つ
- 変更内容、設計判断、検証結果をIssueまたはPull Requestに残す
- 実装方式が未確定の段階でruntime、package manager、frameworkを固定しない

## 実装方式の決め方

- [プロダクト仕様](docs/product-definition.md)を正本とする
- [システム設計](docs/system-design.md)で論理的な責務と処理フローを確認する
- [ドメインモデル](docs/domain-model.md)でデータ関係と状態遷移を確認する
- runtime、framework、DB、queueなどは、候補、理由、影響を整理してから決める
- 技術選定後に、必要なpackage manager、lockfile、migration、実行・検証手順を追加する

## コミットしないもの

- OpenAI APIキー、アクセストークン、Cookie、Authorizationヘッダー
- 実際の保有銘柄、数量、取得価格、資産額、損益
- 個人的なウォッチ設定
- `.env`、ログ、キャッシュ、生成物、ローカルだけの設定

## 設定方針

TODO

<!-- - 公開可能な例は `config/watchlist.example.yml` に置く
- 個人的なウォッチ設定は `config/watchlist.yml` に置き、Gitでは追跡しない
- APIキーはローカルでは環境変数、GitHub ActionsではSecretsから渡す
- サンプルには架空または汎用データだけを使う -->

## エラーハンドリング方針

- 想定内の失敗と想定外のエラーを区別する
- 利用者向け表示、DB上の結果、開発者向けログを分ける
- 取得できなかった情報源はReport内へ記録する
- password、token、secret、未検証のAI生レスポンスを無条件にログへ出さない
- 同じ予定のJobを重複生成せず、Reportと状態更新を部分保存しない
- worker停止をheartbeatで検出できる論理設計を維持する
- 具体的なログ、監視、復旧手順はIssue #7で決める

## インフラ・運用方針

TODO

## 変更前の確認

- 既存変更を `git status` で確認する
- 追加ファイルに秘密情報や個人情報が含まれていないか確認する
- `.gitignore` の対象外に生成物やローカル設定が残っていないか確認する
- 変更した範囲に応じて、構文確認、ビルド、テスト、手動確認を行う
