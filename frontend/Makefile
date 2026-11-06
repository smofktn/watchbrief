include ../make/common.mk

.PHONY: help install dev build typecheck preview audit test-legacy

help:
	@printf '%s\n' \
	  'Frontend — このディレクトリで実行' \
	  '  make install       lockfileに従って依存を導入（install scriptsは無効）' \
	  '  make dev           127.0.0.1:5173で起動' \
	  '  make build         型検査と本番ビルド' \
	  '  make typecheck     Vue/TypeScript型検査' \
	  '  make preview       ビルド成果物のローカル確認（API中継なし）' \
	  '  make audit         npm依存の監査' \
	  '  make test-legacy   旧案の通信試験（参考）'

install:
	$(NPM) ci --ignore-scripts

dev:
	$(NPM) run dev

build:
	$(NPM) run build

typecheck:
	$(NPM) run typecheck

preview:
	$(NPM) run preview

audit:
	$(NPM) audit

test-legacy:
	$(NPM) run test:legacy
