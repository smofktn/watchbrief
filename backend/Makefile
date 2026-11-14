include ../make/common.mk

DB_PORT ?= 5432
export DB_PORT

APP_ORIGIN ?= http://127.0.0.1:5173
API_ADDR ?= 127.0.0.1:8080
API_PASSWORD_FILE ?= ../.local/watchbrief-api-password
LOGIN_ID ?=
RESET_DB_PASSWORD_FILE ?= ../.local/watchbrief-password-resetter-password
DB_PASSWORD_FILE ?= ../.local/watchbrief-user-creator-password
export RESET_DB_PASSWORD_FILE
export APP_ORIGIN API_ADDR API_PASSWORD_FILE LOGIN_ID DB_PASSWORD_FILE

CURRENT_PACKAGES := ./...
COVERAGE_DIR := $(PROJECT_ROOT)/.local/coverage/backend

.PHONY: verify-collection-update verify-collection-creation verify-issue17 help deps dev build build-api build-user-create build-password-reset password-reset verify-password-reset db-provision-password-resetter user-create fmt test coverage test-db benchmark-password check-input verify-user-create verify-login-api verify-logout-api seed-collections verify-collections-api seed-reports verify-reports-api tools-install goose-install sqlc-install sqlc-generate sqlc-vet goose-validate db-prepare db-up db-stop db-status db-bootstrap db-migrate db-migration-status db-version db-verify

help:
	@printf '%s\n' \
	  'Backend — このディレクトリで実行（DB操作もここで管理）' \
	  '  make dev                  API起動（APP_ORIGINで画面Originを指定可能）' \
	  '  make deps                 Go依存をlock情報に従って取得' \
	  '  make build                APIとUser作成・再設定CLIを.local/binへビルド' \
	  '  make verify-collection-update    使い捨てDBで本体編集・競合・再試行・画面を検証' \
	  '  make verify-collection-creation  使い捨てDBで新規保存・同時再試行・画面を検証' \
	  '  make verify-issue17       専用の使い捨てDBで#17を横断検証（Chromium必須）' \
	  '  make password-reset LOGIN_ID=fictional_user  全セッションを無効化して再設定' \
	  '  make db-provision-password-resetter  既存DBへ専用ロール追加（1回だけ）' \
	  '  make verify-password-reset  専用fixtureで再設定と競合を検証' \
	  '  make user-create LOGIN_ID=fictional_user  パスワードは端末で非表示入力' \
	  '  make test / test-db       現在のコードの試験 / 実DBの接続試験' \
	  '  make coverage             全Go packageのcoverageとHTML（実DB試験は別）' \
	  '  make fmt                  現在のGoコードを整形' \
	  '  make benchmark-password / check-input / verify-user-create' \
	  '  make verify-login-api     実DBのログイン試験（検証セッション2件を保持）' \
	  '  make verify-logout-api    実DBのログアウト・障害再試行を検証' \
	  '  make seed-reports LOGIN_ID=fictional_user / verify-reports-api' \
	  '  make seed-collections LOGIN_ID=fictional_user  架空の停止中設定を追加' \
	  '  make verify-collections-api  所有者・404・DB権限の検証' \
	  '  make tools-install / goose-install / sqlc-install' \
	  '  make sqlc-generate / sqlc-vet / goose-validate' \
	  '  make db-prepare / db-up / db-stop / db-status' \
	  '  make db-bootstrap         ロール初期設定（初回のみ、既存は上書きしない）' \
	  '  make db-migrate / db-migration-status / db-version / db-verify'

dev:
	$(GO) run ./cmd/api

deps:
	$(GO) mod download all

build: build-api build-user-create build-password-reset

build-api:
	@mkdir -p "$(PROJECT_ROOT)/.local/bin"
	$(GO) build -o "$(PROJECT_ROOT)/.local/bin/api" ./cmd/api

build-user-create:
	@mkdir -p "$(PROJECT_ROOT)/.local/bin"
	$(GO) build -o "$(PROJECT_ROOT)/.local/bin/user-create" ./cmd/user-create

user-create: build-user-create
	@test -n "$$LOGIN_ID" || { printf '%s\n' 'LOGIN_IDを指定してください（例: make user-create LOGIN_ID=fictional_user）' >&2; exit 2; }
	@"$(PROJECT_ROOT)/.local/bin/user-create" --login-id "$$LOGIN_ID" --db-password-file "$$DB_PASSWORD_FILE"

fmt:
	$(GO) fmt $(CURRENT_PACKAGES)

test:
	$(GO) test -race $(CURRENT_PACKAGES)

coverage:
	@mkdir -p "$(COVERAGE_DIR)"
	$(GO) test -coverprofile="$(COVERAGE_DIR)/coverage.out" $(CURRENT_PACKAGES)
	$(GO) tool cover -func="$(COVERAGE_DIR)/coverage.out"
	$(GO) tool cover -html="$(COVERAGE_DIR)/coverage.out" -o "$(COVERAGE_DIR)/index.html"

test-db:
	$(GO) test -race -tags integration ./internal/adapter/postgres

benchmark-password:
	$(GO) test ./internal/security/password -run '^$$' -bench BenchmarkHash -benchtime=3x -benchmem

# Displays the response; this sample has no real password and should return 400.
check-input:
	@curl --silent --show-error --include http://127.0.0.1:8080/api/login \
	  -H "Origin: $$APP_ORIGIN" -H 'Content-Type: application/json' --data '{}'

verify-user-create: build-user-create
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-user-create.py

seed-collections:
	@test -n "$$LOGIN_ID" || { printf '%s\n' 'LOGIN_IDを指定してください' >&2; exit 2; }
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/seed-collections.py --login-id "$$LOGIN_ID"

verify-collections-api: build-api build-user-create
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-collections-api.py

verify-logout-api: build-api build-user-create
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-logout-api.py

verify-login-api: build-api
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-login-api.py

tools-install: goose-install sqlc-install

goose-install:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/install-goose.py

sqlc-install:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/install-sqlc.py

sqlc-generate:
	"$(PROJECT_ROOT)/.local/bin/sqlc" generate -f sqlc.yaml

sqlc-vet:
	"$(PROJECT_ROOT)/.local/bin/sqlc" vet -f sqlc.yaml

goose-validate:
	"$(PROJECT_ROOT)/.local/bin/goose" -env=none -dir db/goose validate

db-prepare:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/prepare-postgres.py

db-up:
	cd "$(PROJECT_ROOT)" && $(DOCKER) compose up -d --wait postgres

db-stop:
	cd "$(PROJECT_ROOT)" && $(DOCKER) compose stop postgres

db-status:
	cd "$(PROJECT_ROOT)" && $(DOCKER) compose ps

db-bootstrap:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/bootstrap-postgres-roles.py

db-migrate:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/migrate-postgres.py up

db-migration-status:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/migrate-postgres.py status

db-version:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/migrate-postgres.py version

db-verify:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-postgres-schema.py

seed-reports:
	@test -n "$$LOGIN_ID" || { printf '%s\n' 'LOGIN_IDを指定してください' >&2; exit 2; }
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/seed-reports.py --login-id "$$LOGIN_ID"

verify-reports-api: build-api build-user-create
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-reports-api.py

build-password-reset:
	@mkdir -p "$(PROJECT_ROOT)/.local/bin"
	$(GO) build -o "$(PROJECT_ROOT)/.local/bin/password-reset" ./cmd/password-reset

password-reset: build-password-reset
	@test -n "$$LOGIN_ID" || { printf '%s\n' 'LOGIN_IDを指定してください' >&2; exit 2; }
	@"$(PROJECT_ROOT)/.local/bin/password-reset" --login-id "$$LOGIN_ID" --db-password-file "$$RESET_DB_PASSWORD_FILE"

db-provision-password-resetter:
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/provision-password-resetter.py

verify-password-reset: build-api build-user-create build-password-reset
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-password-reset.py
	$(GO) test -race -tags integration ./internal/adapter/postgres -run 'TestPasswordReset'

verify-issue17: build
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-issue17.py

verify-collection-creation: build
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-issue17.py --collection-creation

verify-collection-update: build
	cd "$(PROJECT_ROOT)" && $(PYTHON) scripts/verify-issue17.py --collection-update
