COMPOSE := docker compose -f ops/compose.yml

.PHONY: build-web build-go test dev up down logs reset reset-data reset-media install-hooks help

# ── Build ─────────────────────────────────────────────────────────────────────

build-web: ## Build the React UI into web/dist
	cd web && npm run build

build-go: ## Build the Go binary locally (no UI embed)
	go build -o purser ./cmd/purser

# ── Test ──────────────────────────────────────────────────────────────────────

test: ## Run all Go unit tests
	go test -v ./...

test-integration: ## Run integration tests (requires adapter credential env vars)
	go test -tags integration -timeout 300s -v ./...

install-hooks: ## Install git pre-commit hooks (run once after clone)
	pre-commit install

# ── Dev lifecycle ─────────────────────────────────────────────────────────────

dev: build-web ## Build UI + container image then (re)launch; volumes persist
	$(COMPOSE) down
	docker build --no-cache -f ops/Containerfile -t purser_app:latest .
	$(COMPOSE) up -d

up: ## Start dev stack without rebuilding
	$(COMPOSE) up -d

down: ## Stop dev stack (volumes preserved)
	$(COMPOSE) down

logs: ## Tail app logs
	$(COMPOSE) logs -f app

# ── Volume management ─────────────────────────────────────────────────────────

reset: ## Wipe ALL volumes and restart fresh (database + all media)
	$(COMPOSE) down -v
	$(COMPOSE) up -d

reset-data: ## Wipe only the database volume (keeps downloaded art and content)
	$(COMPOSE) down
	docker volume rm purser_purser-data || true
	$(COMPOSE) up -d

reset-media: ## Wipe only downloaded art/logos (keeps database and content)
	$(COMPOSE) down
	docker volume rm purser_purser-media || true
	$(COMPOSE) up -d

# ── Help ──────────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z][a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-16s %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
