.PHONY: help dev build up down logs logs-all ps migrate shell-db admin-promote \
        test-health playtest-register playtest-login missions-check reset-db

# ── Config ───────────────────────────────────────────────────────────────────
COMPOSE = docker compose
SERVER  = gw-server
DB      = gw-postgres

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ── Development ──────────────────────────────────────────────────────────────
dev: ## Start postgres only (run server locally with go run)
	$(COMPOSE) up postgres -d
	@echo ""
	@echo "PostgreSQL ready. Start the server with:"
	@echo "  cd server && go run ./cmd/server"

build: ## Build the Docker image
	$(COMPOSE) build --no-cache

up: ## Start all services (build if needed)
	$(COMPOSE) up -d --build
	@echo ""
	@echo "Services running. Open http://localhost"

down: ## Stop all services
	$(COMPOSE) down

logs: ## Tail server logs
	$(COMPOSE) logs -f $(SERVER)

logs-all: ## Tail all logs
	$(COMPOSE) logs -f

ps: ## Show running containers
	$(COMPOSE) ps

# ── Database ─────────────────────────────────────────────────────────────────
migrate: ## Apply all SQL migrations in order
	@bash scripts/migrate.sh

shell-db: ## Open psql shell in the postgres container
	docker exec -it $(DB) psql -U $$POSTGRES_USER -d $$POSTGRES_DB

# ── Admin ────────────────────────────────────────────────────────────────────
admin-promote: ## Promote a user to admin: make admin-promote EMAIL=user@example.com
	@[ "$(EMAIL)" ] || ( echo "Usage: make admin-promote EMAIL=user@example.com"; exit 1 )
	docker exec $(DB) psql -U $$POSTGRES_USER -d $$POSTGRES_DB \
	  -c "UPDATE accounts SET is_admin = true WHERE email = '$(EMAIL)';"

# ── Local quick-test helpers (requires curl + jq) ────────────────────────────
BASE ?= http://localhost:8080

test-health: ## Quick server health check
	@curl -sf $(BASE)/admin/health | jq . || echo "Server not reachable or not admin-authed"

playtest-register: ## Register a test account: make playtest-register EMAIL=a@b.com PASS=secret NAME=Alpha
	@curl -s -X POST $(BASE)/auth/register \
	  -H 'Content-Type: application/json' \
	  -d '{"email":"$(EMAIL)","password":"$(PASS)","agent_name":"$(NAME)","faction":"tau_ri"}' | jq .

playtest-login: ## Login and print token: make playtest-login EMAIL=a@b.com PASS=secret
	@curl -s -X POST $(BASE)/auth/login \
	  -H 'Content-Type: application/json' \
	  -d '{"email":"$(EMAIL)","password":"$(PASS)"}' | jq .

missions-check: ## Show active missions for an agent: make missions-check TOKEN=<token>
	@[ "$(TOKEN)" ] || ( echo "Usage: make missions-check TOKEN=<paseto-token>"; exit 1 )
	@curl -s $(BASE)/agent/missions -H "Authorization: Bearer $(TOKEN)" | jq '.missions[] | {title_en, type, progress, target_quantity, status, reward_credits}'

reset-db: ## Drop and recreate the database (WARNING: destroys all data)
	@echo "WARNING: This will destroy all data. Press Ctrl-C to abort, Enter to continue."
	@read _
	docker exec $(DB) psql -U $$POSTGRES_USER -d postgres \
	  -c "DROP DATABASE IF EXISTS $$POSTGRES_DB;" \
	  -c "CREATE DATABASE $$POSTGRES_DB OWNER $$POSTGRES_USER;"
	@echo "Database reset. Restart the server to apply migrations."
