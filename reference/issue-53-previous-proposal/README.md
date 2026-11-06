# Issue #53の以前のAI実装案（参考）

`backend/internal/` は以前のAIが作成した未採用のGoコードを、内容を変えず保存したもの。現在のBackendモジュールの外へ移し、通常のビルド・全体テスト・依存整理に混在させない。実行・保守の対象ではなく、現在の仕様や採用方式の根拠にしない。

現在の実装と操作手順は[Backend](../../backend/README.md)を参照する。旧案の[認証設計](../../docs/auth-implementation.md)・[起動手順](../../docs/local-development.md)・[過去の検証](../../docs/verification-issue-53.md)は当時の配置を記録している。このディレクトリには旧案のauth/config/httpapi/storeだけを保存し、旧SQL・画面等は元の場所に参考として保持している。
