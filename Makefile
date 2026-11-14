include make/common.mk

# Pass the Make executable without marking dev as a recursive recipe: make -n
# must print the supervisor command rather than execute it.
export WATCHBRIEF_DEV_MAKE := $(MAKE)

FRONTEND_COMMANDS := frontend-install frontend-dev frontend-build frontend-typecheck frontend-preview frontend-audit frontend-test frontend-coverage frontend-test-legacy
BACKEND_COMMANDS := backend-deps backend-dev backend-build backend-fmt backend-test backend-coverage backend-test-db backend-benchmark-password backend-check-input
BACKEND_DIRECT := verify-collection-update verify-collection-creation verify-issue17 password-reset verify-password-reset db-provision-password-resetter user-create verify-user-create verify-login-api verify-logout-api seed-collections verify-collections-api seed-reports verify-reports-api tools-install goose-install sqlc-install sqlc-generate sqlc-vet goose-validate db-prepare db-up db-stop db-status db-bootstrap db-migrate db-migration-status db-version db-verify

.PHONY: help dev verify-dev runtime-install runtime-versions build test check coverage $(FRONTEND_COMMANDS) $(BACKEND_COMMANDS) $(BACKEND_DIRECT)

help:
	@printf '%s\n' \
	  'WatchBrief — ルートからの操作' \
	  '  make dev                           DB・API・画面をまとめて起動（Ctrl+CでAPI・画面終了）' \
	  '  make verify-dev                    一括起動の順序・中断・異常終了・多重起動を検証' \
	  '  make frontend-dev / backend-dev     画面・APIを別端末で起動' \
	  '  make frontend-install              Frontend依存を導入' \
	  '  make backend-deps                  Go依存を取得' \
	  '  make build / test / check           両方のビルド / 両方の試験 / 試験と型検査' \
	  '  make verify-collection-update    使い捨てDBで本体編集・競合・再試行・画面を検証' \
	  '  make verify-collection-creation  使い捨てDBでCollection新規保存と再試行を検証' \
	  '  make verify-issue17                使い捨てDBで新規構築・障害復旧・画面を検証' \
	  '  make password-reset LOGIN_ID=fictional_user  非表示入力で再設定・全セッション失効' \
	  '  make db-provision-password-resetter / verify-password-reset' \
	  '  make user-create LOGIN_ID=fictional_user  非表示入力でUser追加' \
	  '  make db-prepare / db-up / db-status / db-stop  DB準備・起動・確認・停止' \
	  '  make db-bootstrap                  ロール初期設定（初回のみ）' \
	  '  make db-migrate / db-migration-status / db-version' \
	  '  make tools-install / sqlc-generate / sqlc-vet / goose-validate' \
	  '  make backend-test-db / db-verify    DB接続・制約・権限の試験' \
	  '  make verify-user-create             PTY試験（検証User1件を保持）' \
	  '  make verify-login-api               DBログイン試験（検証セッション2件を保持）' \
	  '  make verify-logout-api              DBログアウト・旧トークン拒否・障害再試行の試験' \
	  '  make seed-reports LOGIN_ID=fictional_user / verify-reports-api' \
	  '  make seed-collections LOGIN_ID=fictional_user / verify-collections-api' \
	  '  make backend-benchmark-password / backend-check-input' \
	  '  make backend-fmt                    現在のGoコードを整形' \
	  '  make frontend-typecheck / frontend-preview / frontend-audit' \
	  '  make frontend-test                  Vue画面・認証・取得・ルートの試験' \
	  '  make coverage / backend-coverage    両側 / Backendのcoverage（実DB試験は別）' \
	  '  make frontend-coverage             現用Vue/TS全体のV8 coverage' \
	  '  make frontend-test-legacy           旧案の通信試験（参考）' \
	  '  make runtime-install / runtime-versions  asdf版の導入・確認' \
	  'Frontend・Backend側は make -C frontend help / make -C backend help で確認できます。'

$(FRONTEND_COMMANDS): frontend-%:
	+@$(MAKE) --no-print-directory -C frontend $*

$(BACKEND_COMMANDS): backend-%:
	+@$(MAKE) --no-print-directory -C backend $*

# DB recipes live in backend/Makefile as well.
$(BACKEND_DIRECT):
	+@$(MAKE) --no-print-directory -C backend $@

dev:
	@$(PYTHON) scripts/dev.py

verify-dev:
	@$(PYTHON) scripts/verify-dev.py

runtime-install:
	asdf install

runtime-versions:
	asdf current

build: frontend-build backend-build

test: backend-test frontend-test

check: verify-dev backend-test frontend-typecheck frontend-test
	git diff --check

coverage: backend-coverage frontend-coverage
