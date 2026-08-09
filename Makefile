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
	test _test-unit _test-integration test-ci \
	k6 _k6-endpoint _k6-flow k6-ci _k6-app-start _k6-app-stop \
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

test-ci: ## Run the exact suite the PR workflow's Test job runs — the single source of truth for both
	go test -v -race -coverprofile=coverage.out ./...

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

# k6-ci runs the full k6 suite against a standalone purser process — no
# compose stack, no Postgres — see docs/adr/0022-k6-ci-enforcement.md.
# .cidata/ is wiped at the *start* of _k6-app-start, not just cleaned up
# after, so a run is hermetic even if a prior run's teardown was skipped
# (Ctrl-C, CI runner killed mid-job) — it can never inherit state left on
# the box, local or CI runner. PURSER_MUSICBRAINZ_MOCK=1 tells purser serve
# to answer every MusicBrainz call in-process from
# internal/adapters/musicbrainz/fixtureserver's canned data (see
# cmd/purser/serve.go's newMusicIdentificationClients) — every provider
# adapter a k6 test exercises must run against fixture data, never a live
# network call (same reasoning docs/technical/pipeline-music-persist.md's
# Persister tests already apply at the Go level, extended to the k6/CI
# harness here), and no second process/port is needed for it.
# PURSER_PROWLARR_MOCK=1 does the identical thing for
# test/k6/{grpc,http}/indexer_test.js — the generated purser-ci.yaml's
# prowlarr: block enables Prowlarr with placeholder base_url/api_key
# (never dialed; RoundTrip intercepts first) so
# cmd/purser/serve.go's newIndexerSearcher constructs a real
# *prowlarr.Client wired to internal/adapters/prowlarr/fixtureserver's
# canned route table instead of a live Prowlarr instance.
# PURSER_QBITTORRENT_MOCK=1/PURSER_SABNZBD_MOCK=1 do the identical thing for
# test/k6/{grpc,http}/download_test.js — the generated purser-ci.yaml's
# qbittorrent:/sabnzbd: blocks enable both with placeholder credentials
# (never dialed; RoundTrip intercepts first) so cmd/purser/serve.go's
# newQBittorrentClient/newSABnzbdClient construct real clients wired to
# internal/adapters/qbittorrent/fixtureserver's and
# internal/adapters/sabnzbd/fixtureserver's own canned route tables instead
# of live instances. test/k6/flow/
# accept_candidate_test.js scans .cidata/scan-music (real, tagged audio
# fixtures copied from test/k6/fixtures/musicbrainz-audio/ — see that
# directory's own generation notes) against the fixture release
# internal/adapters/musicbrainz/fixtureserver defines. The organize.adult
# entry gives test/k6/grpc/organizer_test.js and test/k6/http/organizer_test.js
# (M11b) a real destination to render into for content_type "adult" — its
# template only uses generic keys (Metadata passthrough + Ext), even though
# AfterDark's own TemplateDataBuilder is registered for "adult" and both
# scripts seed a real Studio LibraryEntry to satisfy it (see
# test/k6/grpc/organizer_test.js's own header comment) — without colliding
# across reruns against a long-lived dev server, since the suite renders a
# fresh, unique k6_marker
# each run. Each of those two scripts gets its own single-file
# .cidata/scan-organize-{grpc,http} root (a real, confirmed CI failure, not
# a hypothetical): both used to default PURSER_SCAN_FIXTURE_ROOT to the same
# .cidata/scan test/k6/{grpc,http}/scan_test.js and
# test/k6/{grpc,http}/unmatched_file_test.js also scan, and Organize
# physically moves the file it's given — whichever of scan_test.js/
# unmatched_file_test.js runs after organizer_test.js in the same
# _k6-endpoint loop then walked a directory one file short of what it
# expected (3 fixture files became 2), failing scan_test.js's own "3 tasks"
# check. grpc and http additionally can't share even their own isolated
# root with each other: _k6-endpoint runs every test/k6/grpc/*.js file
# before any test/k6/http/*.js file, so if both organizer_test.js variants
# pointed at the same single-file root, the http run would find the file
# already moved out from under it by the grpc run moments earlier. The
# organize.music entry gives test/k6/flow/organize_music_test.js a
# real destination to manually Organize into, proving Music's actual
# TemplateDataBuilder renders every real field (ArtistName/AlbumTitle/Year/
# DiscCount/TrackTitle/Ext) end-to-end against a live server — nothing did
# before (organizer_test.js deliberately avoids TemplateDataBuilder
# entirely by using content_type "adult"). That flow scans its own
# isolated .cidata/scan-music-organize root (fixtures copied from
# test/k6/fixtures/musicbrainz-audio/organize/ — same MBID/tags as
# scan-music's own 01/02 so it identifies and auto-imports exactly the
# same way, but genuinely distinct audio bytes/hash, generated
# specifically so this never collides with accept_candidate_test*.js's
# own files) rather than reusing scan-music directly: Organize physically
# moves the file, and accept_candidate_test.js/accept_candidate_test_http.js
# both scan the exact same scan-music fixtures later in the same
# k6-ci invocation — organizing one out from under it would silently break
# whichever of those two runs second. auto_organize itself stays off
# (unset/default false) for the same reason at the config level: it's a
# single global toggle, so leaving it off and calling Organize explicitly
# in the one flow that wants it is what keeps this safe, rather than
# trying to scope AutoOrganize's effect to just one scan root. No
# printf-template-function %02d zero-padding here (unlike the documented
# default template) — avoiding a doubly-escaped % and nested quote inside
# this already-single-quoted shell printf isn't worth it just to prove
# wiring; zero-padding itself is proven separately (Go unit tests, and
# this milestone's own manual verification).
_k6-app-start: $(GOBIN)/k6
	rm -rf .cidata
	mkdir -p .cidata
	mkdir -p .cidata/scan
	@for f in one two three; do head -c 70000 /dev/urandom > .cidata/scan/$$f.bin; done
	mkdir -p .cidata/scan-organize-grpc .cidata/scan-organize-http
	@head -c 70000 /dev/urandom > .cidata/scan-organize-grpc/fixture.bin
	@head -c 70000 /dev/urandom > .cidata/scan-organize-http/fixture.bin
	mkdir -p .cidata/scan-music/ambiguous
	cp test/k6/fixtures/musicbrainz-audio/*.flac .cidata/scan-music/
	cp test/k6/fixtures/musicbrainz-audio/ambiguous/*.flac .cidata/scan-music/ambiguous/
	mkdir -p .cidata/scan-music-organize
	cp test/k6/fixtures/musicbrainz-audio/organize/*.flac .cidata/scan-music-organize/
	printf 'pipeline:\n  scan_roots:\n    - path: %s/.cidata/scan-music\n      content_type: music\n    - path: %s/.cidata/scan-music-organize\n      content_type: music\n  organize:\n    adult:\n      root: %s/.cidata/organized\n      template: "{{.Metadata.k6_marker}}{{.Ext}}"\n    music:\n      root: %s/.cidata/organized-music\n      template: "{{.ArtistName}}/{{.AlbumTitle}}{{if .Year}} ({{.Year}}){{end}}/{{if gt .DiscCount 1}}{{.DiscNumber}}-{{end}}{{.TrackNumber}} - {{.TrackTitle}}{{.Ext}}"\nprowlarr:\n  enabled: true\n  base_url: http://prowlarr.invalid/api/v1\n  api_key: k6-fixture-key\nqbittorrent:\n  enabled: true\n  base_url: http://qbittorrent.invalid\n  username: k6-fixture-user\n  password: k6-fixture-pass\nsabnzbd:\n  enabled: true\n  base_url: http://sabnzbd.invalid\n  api_key: k6-fixture-key\n' "$(CURDIR)" "$(CURDIR)" "$(CURDIR)" "$(CURDIR)" > .cidata/purser-ci.yaml
	go build -o .cidata/purser ./cmd/purser
	PURSER_PATHS_DATA_DIR=$(CURDIR)/.cidata/data PURSER_MUSICBRAINZ_MOCK=1 PURSER_PROWLARR_MOCK=1 PURSER_QBITTORRENT_MOCK=1 PURSER_SABNZBD_MOCK=1 .cidata/purser serve --config $(CURDIR)/.cidata/purser-ci.yaml & echo $$! > .cidata/purser.pid
	@for i in $$(seq 1 60); do nc -z localhost 7474 2>/dev/null && exit 0; sleep 0.5; done; \
		echo "purser serve did not come up on :7474 within 30s" >&2; exit 1

_k6-app-stop:
	@if [ -f .cidata/purser.pid ]; then \
		pid="$$(cat .cidata/purser.pid)"; \
		kill "$$pid" 2>/dev/null || true; \
		for i in $$(seq 1 20); do kill -0 "$$pid" 2>/dev/null || break; sleep 0.5; done; \
	fi
	rm -rf .cidata

# PURSER_SCAN_FIXTURE_ROOT overrides test/k6/{grpc,http}/scan_test.js's
# compose-oriented default (/media/content/scan) with the hermetic fixture
# _k6-app-start just created under .cidata/scan — see
# docs/adr/0024-pipeline-core.md. PURSER_SCAN_MUSIC_FIXTURE_ROOT does the
# same for test/k6/flow/accept_candidate_test*.js's own tagged-audio
# fixture root — see docs/technical/pipeline-music-persist.md.
# PURSER_SCAN_MUSIC_ORGANIZE_FIXTURE_ROOT does the same for
# test/k6/flow/organize_music_test.js's own isolated fixture root — see
# that flow's own header comment for why it can't reuse scan-music.
# PURSER_SCAN_ORGANIZE_GRPC_FIXTURE_ROOT / PURSER_SCAN_ORGANIZE_HTTP_FIXTURE_ROOT
# give test/k6/{grpc,http}/organizer_test.js their own single-file roots —
# see _k6-app-start's own comment for why reusing .cidata/scan (or each
# other's root) isn't safe once Organize is in the picture.
k6-ci: _k6-app-start ## Build+run the app standalone (Badger, telemetry off, hermetic .cidata/) and run the full k6 suite against it — no compose stack needed
	@PURSER_SCAN_FIXTURE_ROOT=$(CURDIR)/.cidata/scan PURSER_SCAN_MUSIC_FIXTURE_ROOT=$(CURDIR)/.cidata/scan-music PURSER_SCAN_MUSIC_ORGANIZE_FIXTURE_ROOT=$(CURDIR)/.cidata/scan-music-organize PURSER_SCAN_ORGANIZE_GRPC_FIXTURE_ROOT=$(CURDIR)/.cidata/scan-organize-grpc PURSER_SCAN_ORGANIZE_HTTP_FIXTURE_ROOT=$(CURDIR)/.cidata/scan-organize-http $(MAKE) k6; status=$$?; $(MAKE) _k6-app-stop; exit $$status

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
	@echo "  k6-ci                 Build+run purser standalone (Badger, hermetic .cidata/) and run the full k6 suite — no compose stack"
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
