COMPOSE := docker compose -f ops/compose.yml

GOBIN := $(CURDIR)/.gobin
BUF_VERSION := v1.71.0
PROTOC_GEN_GO_VERSION := v1.36.11
PROTOC_GEN_CONNECT_GO_VERSION := v1.20.0
K6_VERSION := v1.8.0

.PHONY: build-web build-go test test-integration install-hooks tools proto-gen k6-grpc k6-http k6 dev up down logs reset reset-data reset-media help

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

# ── API tooling (proto/gRPC/k6) ──────────────────────────────────────────────
# Pinned dev tools install into ./.gobin, never into go.mod/go.sum — see
# docs/adr/0011-api-design.md.

$(GOBIN)/buf:
	GOBIN=$(GOBIN) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

$(GOBIN)/protoc-gen-go:
	GOBIN=$(GOBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)

$(GOBIN)/protoc-gen-connect-go:
	GOBIN=$(GOBIN) go install connectrpc.com/connect/cmd/protoc-gen-connect-go@$(PROTOC_GEN_CONNECT_GO_VERSION)

$(GOBIN)/k6:
	GOBIN=$(GOBIN) go install go.k6.io/k6@$(K6_VERSION)

tools: $(GOBIN)/buf $(GOBIN)/protoc-gen-go $(GOBIN)/protoc-gen-connect-go $(GOBIN)/k6 ## Install pinned buf/protoc-gen-*/k6 into .gobin

proto-gen: $(GOBIN)/buf $(GOBIN)/protoc-gen-go $(GOBIN)/protoc-gen-connect-go ## Generate Go code from proto/ via buf
	PATH="$(GOBIN):$$PATH" $(GOBIN)/buf generate

k6-grpc: $(GOBIN)/k6 ## Run the gRPC k6 suite against a running server (PURSER_GRPC_ADDR)
	@for f in test/k6/grpc/*.js; do $(GOBIN)/k6 run "$$f" || exit 1; done

k6-http: $(GOBIN)/k6 ## Run the HTTP/JSON (Connect) k6 suite against a running server
	@for f in test/k6/http/*.js; do $(GOBIN)/k6 run "$$f" || exit 1; done

k6: k6-grpc k6-http ## Run both k6 suites

# ── Dev lifecycle ─────────────────────────────────────────────────────────────

dev: ## Build UI locally, then rebuild container image from scratch and (re)launch
	$(COMPOSE) down
	cd web && npm run build
	$(COMPOSE) build --no-cache --pull
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
