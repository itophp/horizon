SHELL := /bin/sh
-include .env
export
.PHONY: install tools dev dev-api db-up db-down migrate generate check-generated format format-check lint typecheck test build test-integration test-e2e test-webmcp-e2e check-rules ci-fast test-security bootstrap ci
install:
	pnpm install --frozen-lockfile

tools:
	GOBIN="$(CURDIR)/.tools/bin" ./scripts/go.sh install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

bootstrap:
	./scripts/go.sh run ./cmd/bootstrap

dev:
	pnpm dev

dev-api:
	./scripts/go.sh run ./cmd/api

db-up:
	docker compose up -d --wait

db-down:
	docker compose stop

migrate:
	./scripts/go.sh run ./cmd/migrate

generate:
	./scripts/go.sh run goa.design/goa/v3/cmd/goa gen baseline/services/api/design
	cd services/api && ../../.tools/bin/sqlc generate
	pnpm --filter @baseline/web generate
	pnpm --filter @baseline/web exec prettier --write src/api/generated

check-generated:
	node scripts/check-generated.mjs

format:
	pnpm format
	./scripts/check-gofmt.sh --write

format-check:
	pnpm format:check
	./scripts/check-gofmt.sh

lint:
	pnpm lint
	./scripts/go.sh vet ./...

typecheck:
	pnpm typecheck

test:
	pnpm test
	./scripts/go.sh test ./...
	pnpm i18n:check

build:
	pnpm build
	./scripts/go.sh build ./...

test-integration:
	./scripts/go.sh test -tags=integration -count=1 -v ./tests/...

E2E_ARGS ?=
test-e2e:
	pnpm test:e2e $(E2E_ARGS)

test-webmcp-e2e:
	pnpm test:e2e webmcp.spec.ts

check-rules:
	pnpm rules:check

ci-fast:
	$(MAKE) check-rules format-check lint typecheck test check-generated build

test-security:
	./scripts/go.sh test ./internal/auth ./internal/authorization ./internal/config ./internal/dataaccess ./internal/identity ./internal/transport
	$(MAKE) test-integration

ci:
	$(MAKE) ci-fast
	$(MAKE) test-integration test-webmcp-e2e test-e2e
