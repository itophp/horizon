SHELL := /bin/sh
-include .env
export
.PHONY: install tools dev dev-api db-up db-down migrate generate check-generated format format-check lint typecheck test build test-integration test-e2e test-e2e-smoke test-e2e-changed test-e2e-parallel test-webmcp-e2e check-rules ci-fast test-security bootstrap ci
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

# 日常最小浏览器验证：@smoke 只覆盖 API→SDK→UI 与登录会话两条最便宜链路。
test-e2e-smoke:
	pnpm test:e2e --grep @smoke

# 按 git 改动选相关文件；纯文档/规则改动直接不启动浏览器。
test-e2e-changed:
	node scripts/e2e-changed.mjs

# 两套独立栈并行跑完整集合，端口与产物不冲突。
test-e2e-parallel:
	./scripts/e2e-parallel.sh

test-webmcp-e2e:
	pnpm test:e2e webmcp.spec.ts

check-rules:
	pnpm rules:check

ci-fast:
	$(MAKE) check-rules format-check lint typecheck test check-generated build

test-security:
	./scripts/go.sh test ./internal/auth ./internal/authorization ./internal/config ./internal/dataaccess ./internal/identity ./internal/mcp ./internal/transport
	$(MAKE) test-integration

ci:
	$(MAKE) ci-fast
	$(MAKE) test-integration test-webmcp-e2e test-e2e

# Remote business MCP: protocol/database security and isolated browser chain.
test-mcp:
	pnpm mcp:test
	./scripts/go.sh test ./internal/mcp
	./scripts/go.sh test -tags=integration -count=1 -run MCP ./tests/...

test-mcp-e2e:
	pnpm test:e2e "e2e/mcp.spec.ts$$"

# Requires real local Codex and WorkBuddy CLI; never silently skipped.
test-mcp-native-e2e:
	MCP_NATIVE_ACCEPTANCE=1 pnpm test:e2e "e2e/mcp.spec.ts$$"

test-mcp-native:
	MCP_NATIVE_ACCEPTANCE=1 ./scripts/go.sh test -tags=integration -count=1 -timeout=3m -run TestMCPNativeClients -v ./tests/...
