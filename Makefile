.PHONY: run run-agent run-agent-with-config sync-env-from-grapery \
        build test lint clean docker docker-up docker-down help

# ========== Port Configuration ==========
# agent: 9020 (default) — 与 grapery Makefile 中 AGENT_PORT 一致

AGENT_PORT ?= 9020
GRAPERY_BASE_URL ?= http://localhost:8080

# ========== Individual Service Run Commands ==========

run: run-agent

run-agent:
	@echo "🚀 Starting Grapery Agent on port $(AGENT_PORT)..."
	@echo "   Tip: [ ! -f .env ] && cp env.grapery-agent.dev.example .env  (won't overwrite)"
	@echo "   Or:  make sync-env-from-grapery  (create if missing; only fill empty keys)"
	@set -a; \
	cli_huoshan="$${HUOSHAN_API_KEY-}"; cli_gemini="$${GEMINI_API_KEY-}"; \
	[ -f .env ] && . ./.env; \
	[ -n "$$cli_huoshan" ] && export HUOSHAN_API_KEY="$$cli_huoshan"; \
	[ -n "$$cli_gemini" ] && export GEMINI_API_KEY="$$cli_gemini"; \
	set +a; \
	provider="$${EINO_TEXT_PROVIDER:-huoshan}"; \
	if [ "$$provider" = "huoshan" ] && [ -z "$${HUOSHAN_API_KEY:-}" ]; then \
		echo "❌ HUOSHAN_API_KEY is missing in .env (required when EINO_TEXT_PROVIDER=huoshan)"; \
		echo "   Set it in grapery-agent/.env or grapery/.env then: make sync-env-from-grapery"; \
		exit 1; \
	fi; \
	if [ "$$provider" = "gemini" ] && [ -z "$${GEMINI_API_KEY:-}" ]; then \
		echo "❌ GEMINI_API_KEY is missing in .env (required when EINO_TEXT_PROVIDER=gemini)"; \
		exit 1; \
	fi; \
	if [ -z "$${AGENT_TOKEN_VERIFY_KEY:-}" ] && [ -n "$${AGENT_TOKEN_SIGNING_KEY:-}" ]; then \
		export AGENT_TOKEN_VERIFY_KEY="$${AGENT_TOKEN_SIGNING_KEY}"; \
	fi; \
	if [ -z "$${GRAPERY_API_KEY:-}" ] && [ -n "$${GRAPERY_INTERNAL_API_KEY:-}" ]; then \
		export GRAPERY_API_KEY="$${GRAPERY_INTERNAL_API_KEY}"; \
	fi; \
	SERVER_PORT=$(AGENT_PORT) \
	GRAPERY_BASE_URL=$${GRAPERY_BASE_URL:-$(GRAPERY_BASE_URL)} \
	go run ./cmd/server

run-agent-with-config:
	@if [ ! -f .env ]; then \
		echo "⚠️  .env not found, creating from example..."; \
		cp env.grapery-agent.dev.example .env; \
		echo "✅ .env created. Please edit it with your settings."; \
	else \
		echo "✅ .env already exists (not overwritten)"; \
	fi
	@$(MAKE) run-agent AGENT_PORT=$(AGENT_PORT) GRAPERY_BASE_URL=$(GRAPERY_BASE_URL)

# 从 grapery/.env 拉取 HUOSHAN_* / AGENT_TOKEN_* 等填入本仓库 .env（需 sibling 存在）
sync-env-from-grapery:
	bash scripts/sync_env_from_grapery.sh

# ========== Build Commands ==========

build:
	@echo "🔨 Building Grapery Agent..."
	@mkdir -p bin
	go build -o bin/grapery-agent ./cmd/server
	@echo "✅ Built: bin/grapery-agent"

# ========== Development Tools ==========

lint:
	gofmt -w .
	go vet ./...

test:
	go test ./...

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	@echo "✅ Clean complete"

# ========== Docker ==========

docker:
	docker build -t grapery-agent:local .

docker-up:
	@echo "🐳 Starting grapery-agent via compose (needs grapery-network)..."
	@echo "   Tip: [ ! -f .env ] && cp env.grapery-agent.dev.example .env"
	docker network create grapery-network 2>/dev/null || true
	docker compose -f docker-compose.grapery-agent.yml -p grapery-agent up -d grapery-agent

docker-down:
	docker compose -f docker-compose.grapery-agent.yml -p grapery-agent down

# ========== Help ==========

help:
	@echo "Grapery Agent Makefile"
	@echo ""
	@echo "Services:"
	@echo "  agent    - Grapery Agent (port $(AGENT_PORT))"
	@echo ""
	@echo "Run Commands:"
	@echo "  make run / make run-agent   - Run agent (port $(AGENT_PORT), loads .env if present)"
	@echo "  make run-agent-with-config  - Create .env from example only if missing, then run"
	@echo "  make sync-env-from-grapery  - Create .env if missing; fill empty keys from grapery/.env"
	@echo ""
	@echo "Build Commands:"
	@echo "  make build                  - Build bin/grapery-agent"
	@echo ""
	@echo "Development:"
	@echo "  make lint                   - Format and vet"
	@echo "  make test                   - Run tests"
	@echo "  make clean                  - Remove bin/"
	@echo ""
	@echo "Docker:"
	@echo "  make docker                 - Build local image"
	@echo "  make docker-up              - Compose up (grapery-network)"
	@echo "  make docker-down            - Compose down"
	@echo ""
	@echo "Example with custom settings:"
	@echo "  make run-agent AGENT_PORT=9020 GRAPERY_BASE_URL=http://localhost:8080"
	@echo ""
	@echo "Agent (.env or env vars, see env.grapery-agent.dev.example):"
	@echo "  HUOSHAN_API_KEY / EINO_TEXT_MODEL / HUOSHAN_* — AI"
	@echo "  AGENT_TOKEN_VERIFY_KEY — must match grapery AGENT_TOKEN_SIGNING_KEY"
	@echo "  GRAPERY_API_KEY — must match grapery GRAPERY_INTERNAL_API_KEY"
	@echo "  GRAPERY_BASE_URL — local default http://localhost:8080"
