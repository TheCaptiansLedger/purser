COMPOSE := docker compose -f ops/compose.yml --env-file .env

GOBIN := $(CURDIR)/.gobin
BUF_VERSION := v1.71.0
PROTOC_GEN_GO_VERSION := v1.36.11
PROTOC_GEN_CONNECT_GO_VERSION := v1.20.0
K6_VERSION := v1.8.0
GORELEASER_VERSION := v2.17.0

# Namespaces (build/test/k6/compose) dispatch on the extra bare word(s)
# after the target name, e.g. `make test unit`. GNU Make's own flag
# parsing swallows any unquoted `-xyz` word before it ever reaches
# MAKECMDGOALS (verified against the GNU Make 3.81 macOS ships) — build/
# test/k6 never need one, so plain unquoted words are always safe there.
# compose does need real docker-compose flags sometimes; see its recipe.
.PHONY: build _build-web _build-go _build-image \
	test _test-unit _test-integration \
	k6 _k6-endpoint _k6-flow \
	compose reset dev run \
	install-hooks tools proto-gen help

# ── Build ─────────────────────────────────────────────────────────────────────
# Goreleaser is the only thing that builds the Go binary, locally and in CI —
# see docs/adr/0017-build-and-release-goreleaser.md.

build: ## Build web UI + Go binary (host OS/arch). Subcommands: web, go, image
	@case "$(filter-out build,$(MAKECMDGOALS))" in \
	  "") $(MAKE) _build-web && $(MAKE) _build-go ;; \
	  web) $(MAKE) _build-web ;; \
	  go) $(MAKE) _build-go ;; \
	  image) $(MAKE) _build-image ;; \
	  *) echo "usage: make build [web|go|image]" >&2; exit 1 ;; \
	esac

_build-web:
	cd web && npm run build

_build-go: _build-web $(GOBIN)/goreleaser
	$(GOBIN)/goreleaser build --single-target --snapshot --clean
	cp "$$(find dist -type f -name purser | head -1)" ./purser

_build-image: _build-web $(GOBIN)/goreleaser
	GOOS=linux GOARCH=amd64 $(GOBIN)/goreleaser build --single-target --snapshot --clean
	cp "$$(find dist -type f -name purser | head -1)" ./purser
	$(COMPOSE) build app; status=$$?; rm -f ./purser; exit $$status

# ── Test ──────────────────────────────────────────────────────────────────────

test: ## Run every Go test suite (unit + integration). Subcommands: unit, integration
	@case "$(filter-out test,$(MAKECMDGOALS))" in \
	  "") $(MAKE) _test-unit && $(MAKE) _test-integration ;; \
	  unit) $(MAKE) _test-unit ;; \
	  integration) $(MAKE) _test-integration ;; \
	  *) echo "usage: make test [unit|integration]" >&2; exit 1 ;; \
	esac

_test-unit:
	go test -v ./...

_test-integration: ## Run integration tests (requires adapter credential env vars)
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

$(GOBIN)/goreleaser:
	GOBIN=$(GOBIN) go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

tools: $(GOBIN)/buf $(GOBIN)/protoc-gen-go $(GOBIN)/protoc-gen-connect-go $(GOBIN)/k6 $(GOBIN)/goreleaser ## Install pinned buf/protoc-gen-*/k6/goreleaser into .gobin

proto-gen: $(GOBIN)/buf $(GOBIN)/protoc-gen-go $(GOBIN)/protoc-gen-connect-go ## Generate Go code from proto/ via buf
	PATH="$(GOBIN):$$PATH" $(GOBIN)/buf generate

k6: $(GOBIN)/k6 ## Run every k6 suite against a running server. Subcommands: endpoint, flow
	@case "$(filter-out k6,$(MAKECMDGOALS))" in \
	  "") $(MAKE) _k6-endpoint && $(MAKE) _k6-flow ;; \
	  endpoint) $(MAKE) _k6-endpoint ;; \
	  flow) $(MAKE) _k6-flow ;; \
	  *) echo "usage: make k6 [endpoint|flow]" >&2; exit 1 ;; \
	esac

_k6-endpoint: $(GOBIN)/k6
	@for f in test/k6/grpc/*.js test/k6/http/*.js; do $(GOBIN)/k6 run "$$f" || exit 1; done

_k6-flow: $(GOBIN)/k6
	@if ls test/k6/flow/*.js >/dev/null 2>&1; then \
		for f in test/k6/flow/*.js; do $(GOBIN)/k6 run "$$f" || exit 1; done; \
	else \
		echo "no k6 flow tests yet (test/k6/flow/*.js) — skipping"; \
	fi

# ── Local dev stack (Postgres + Grafana + Prometheus + Tempo [+ app]) ────────
# One compose file, one Postgres instance — see
# docs/adr/0018-local-development-environment.md. `app` sits behind the
# "app" compose profile: bare `up` starts only the observability stack, so
# you can run the app natively (`make run`) against it instead — use
# `make dev` when you want the containerized app included too.

compose: ## Manage the observability stack (Postgres/Grafana/Prometheus/Tempo). Subcommands: up, down, logs, ps, or any docker compose command
	@args="$(filter-out compose,$(MAKECMDGOALS))"; \
	case "$$args" in \
	  "") echo "usage: make compose <up|down|logs|ps|...>" >&2; \
	      echo "docker compose flags need quoting: make compose \"logs -f app\"" >&2; exit 1 ;; \
	  up) $(COMPOSE) up -d ;; \
	  down|down\ *) $(COMPOSE) --profile app $$args ;; \
	  *) $(COMPOSE) $$args ;; \
	esac

# All local dev state lives under .local/<component>/ — see
# docs/adr/0018-local-development-environment.md. reset just deletes; it
# does not touch the compose stack.

reset: ## Delete local dev state. Subcommands: data, media, pgsql, grafana, prometheus, tempo (bare = everything under .local)
	@case "$(filter-out reset,$(MAKECMDGOALS))" in \
	  "") rm -rf .local ;; \
	  data) rm -rf .local/data ;; \
	  media) rm -rf .local/media ;; \
	  pgsql) rm -rf .local/postgres ;; \
	  grafana) rm -rf .local/grafana ;; \
	  prometheus) rm -rf .local/prometheus ;; \
	  tempo) rm -rf .local/tempo ;; \
	  *) echo "usage: make reset [data|media|pgsql|grafana|prometheus|tempo]" >&2; exit 1 ;; \
	esac

dev: ## Rebuild the app image from scratch and (re)launch the whole stack, app included (--profile app)
	$(MAKE) build image
	$(COMPOSE) --profile app up -d

run: ## Run the server natively (go run), sourcing .env the same way the container does
	@if [ -f .env ]; then set -a; . ./.env; set +a; go run ./cmd/purser serve; else go run ./cmd/purser serve; fi

# ── Help ──────────────────────────────────────────────────────────────────────

help: ## Show this help
	@echo "Namespaces — run bare for the default action, or with a subcommand:"
	@echo "  build                 Build web UI + Go binary (host OS/arch)"
	@echo "    build web           Build the React UI into web/dist"
	@echo "    build go            Build the local Go binary (./purser)"
	@echo "    build image         Build the local single-arch (linux/amd64) container image"
	@echo "  test                  Run every Go test suite (unit + integration)"
	@echo "    test unit           Run Go unit tests"
	@echo "    test integration    Run integration tests (requires adapter credential env vars)"
	@echo "  k6                    Run every k6 suite against a running server (endpoint + flow)"
	@echo "    k6 endpoint         Run the k6 single-endpoint suites (grpc + http)"
	@echo "    k6 flow             Run the k6 multi-step flow suite (no-ops until it exists)"
	@echo "  compose <cmd>         Manage Postgres/Grafana/Prometheus/Tempo (app NOT included — run it natively with"
	@echo "                        'make run', or use 'make dev' for the containerized app too): up, down, logs, ps,"
	@echo "                        or any docker compose command — flags need quoting: make compose \"logs -f app\""
	@echo "  reset                 Delete ALL local dev state (.local/) — does not touch the compose stack"
	@echo "    reset data          Delete only the datastore directory"
	@echo "    reset media         Delete only downloaded art/logos"
	@echo "    reset pgsql         Delete only the Postgres data directory"
	@echo "    reset grafana       Delete only Grafana's config/state"
	@echo "    reset prometheus    Delete only Prometheus's TSDB"
	@echo "    reset tempo         Delete only Tempo's trace storage"
	@echo
	@echo "Other targets:"
	@grep -E '^[a-zA-Z][a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*##"}; {printf "  %-16s %s\n", $$1, $$2}'

# Lets `make build web`/`make test unit`/`make compose up`/... forward their
# extra word(s) as MAKECMDGOALS instead of make trying (and failing) to find
# a target with that literal name.
%:
	@:

.DEFAULT_GOAL := help
