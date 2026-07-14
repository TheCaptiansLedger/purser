# 0011. API Design: Connect-Primary RPC API

Status: Accepted

## Context

Purser is API-first: the web UI and any future external consumer talk to the
server only through this layer, never through a shared database or internal
package import. `docs/technical/shared-domain-model.md` (Part 5, item 3)
flagged this gap explicitly when the shared kernel domain types were
designed: *"the actual API contract design (proto shapes,
REST-via-gateway or hand-rolled, pagination conventions) is unaddressed and
should get its own ADR before that layer is built."* This is that ADR.

The starting requirements, from the planning conversation:

- gRPC is the API to build first. It's a **driving adapter** to an internal
  service layer, per [0001](0001-hexagonal-architecture.md) — never the
  thing business logic is written against.
- HTTP/JSON is a second transport the same service layer must support, on a
  timeline TBD. It does not get its own handler layer or its own service
  logic — whatever makes this "free" or near-free is preferred over building
  two API surfaces.
- The web UI talks **RPC** (Connect protocol / gRPC-Web via a
  connect-web-generated client), not HTTP/JSON — HTTP/JSON is for other
  consumers (curl, k6, tooling), not the primary UI path.
- Endpoints are **narrow and SRP by construction**: a "create a Person" RPC
  knows how to persist a `Person`, nothing else. It does not reach out to a
  metadata provider, does not enrich its input, and does not know any
  provider exists. If a caller needs metadata to build a complete `Person`,
  that's a separate call to a separate (future) metadata-provider RPC — the
  caller assembles the entity, the write RPC only persists what it's given.
  The one exception is `id` on `Create`: per
  [0020](0020-server-generated-kernel-entity-ids.md), every single-ID
  kernel entity's `ID` is server-generated and any caller-supplied value is
  discarded, not persisted.
  This mirrors [0001](0001-hexagonal-architecture.md)'s rule that
  content-type/provider knowledge never leaks into shared code, applied to
  the API layer instead of the service layer.
- Every RPC gets a k6 test, in both wire protocols this ADR settles on.

No metadata-provider adapters exist yet and none are needed to build this
layer — every RPC in this pass operates purely on data the caller supplies.

## Decision

### Framework: Connect, not vanilla gRPC or REST

**[connectrpc.com/connect](https://connectrpc.com)** is the only RPC
framework. One generated service implementation, from one `.proto` file,
simultaneously serves gRPC, gRPC-Web, and Connect's own HTTP/JSON protocol —
there is no second handler layer for HTTP/JSON, no `grpc-gateway`, no
hand-rolled REST controllers. This satisfies "HTTP/JSON down the road for
free" literally: the code already exists the moment the gRPC service does.

**Caveat, stated explicitly so it isn't rediscovered later:** Connect's
HTTP/JSON is RPC-shaped, not resource-oriented REST — e.g.
`POST /purser.domain.v1.PersonService/GetPerson` with a JSON body, not
`GET /persons/123`. This is accepted as correct for this project: the web UI
uses real RPC (Connect protocol), and HTTP/JSON exists for k6/tooling/future
non-UI consumers, not to be a REST API.

OpenAPI/Swagger documentation, when wanted, is generated from the same
`.proto` files via a Connect-aware plugin (e.g. `protoc-gen-connect-openapi`)
rather than hand-maintained — not wired up in this pass, flagged as a
follow-up once the first services exist to document.

### Tooling: pinned, Makefile-driven, outside go.mod

`buf`, `protoc-gen-go`, `protoc-gen-connect-go`, and `k6` are installed by
`make tools` into a local `.gobin/` (gitignored), at versions pinned as
Makefile variables — **not** as `go.mod` `tool` directives. Tried the latter
first: `go get -tool github.com/bufbuild/buf/cmd/buf` alone pulled ~90
transitive modules into `go.sum` (Docker client, quic-go, cel-go — features
this project never uses, needed only for buf's BSR/plugin-execution
surface). `go install <pkg>@<version>` run outside module-resolution mode
installs the same pinned binary without touching `go.mod`/`go.sum` at all —
confirmed by diffing both files before/after. `make proto-gen` and
`make k6`/`make k6-grpc`/`make k6-http` depend on `make tools` so the
binaries always exist before use.

### Layout

```
proto/purser/domain/v1/*.proto      # one file per shared-kernel entity
proto/purser/afterdark/v1/*.proto   # AfterDark module-specific types
                                     # (future modules get their own
                                     # proto/purser/<module>/v1/)
gen/go/...                          # generated code, committed to the repo
                                     # (CI/`go build` never require buf to
                                     # be installed just to compile)
internal/ports/...                  # one narrow repository interface per
                                     # entity, per 0001/0002
internal/service/...                # one service per entity — orchestrates
                                     # domain + ports, zero proto/Connect
                                     # knowledge
internal/api/connect/...            # generated-interface implementations:
                                     # thin request/response translation,
                                     # interceptors, error mapping — the
                                     # driving adapter itself
cmd/purser/...                      # composition root: wires config,
                                     # adapters, services, interceptors,
                                     # starts the Connect/h2c server
test/k6/grpc/...                    # one script per RPC, k6/net/grpc,
                                     # loads the .proto directly
test/k6/http/...                    # one script per RPC, k6's http module
                                     # against the same RPC's Connect
                                     # HTTP/JSON endpoint
```

### One Connect service per kernel entity (SRP)

Each shared-kernel entity gets its own proto service and its own
`internal/service` type: `PersonService`, `LibraryEntryService`,
`GroupService`, `ItemService`, `EntryPersonService`, `ItemPersonService`,
`TagService`, `ExternalIDService`, `ImageService`, `MediaFileService`, plus
module-specific services as modules are added (e.g.
`afterdark.PerformerProfileService`). No handler or service imports another
entity's port. A handler assembling a composed view (e.g. a `PerformerView`
per `shared-domain-model.md` Part 3) is a **future, separate, explicitly
composing** service — never folded into a single-entity CRUD service.

Standard method set per entity, mirrored from AIP resource conventions
without being dogmatic about it: `Create<Entity>`, `Get<Entity>`,
`Update<Entity>` (takes a `google.protobuf.FieldMask` for partial updates —
avoids accidental clobber of fields the caller didn't intend to touch),
`Delete<Entity>`, `List<Entity>` (see pagination below). Join-shaped
entities (`EntryPerson`, `ItemPerson`, `ExternalID`, `Image`) additionally
get a `List<Entity>By<Parent>` filtered lookup — still a read of their own
type, not a reach into the parent's service.

### Pagination

`page_size` + opaque `page_token` (cursor), response includes
`next_page_token`. No offset/limit anywhere — cursor pagination is stable
under concurrent writes, offset pagination isn't.

### Error mapping

`internal/api/connect` has one shared error-mapping helper, not ad hoc
`connect.NewError` calls scattered per handler:

- `domain.ValidationError` (already designed in `validate.go` specifically
  to "survive up to an API error response later") → `connect.CodeInvalidArgument`,
  with each `FieldError` attached as a structured error detail.
- New sentinel errors `ports.ErrNotFound` / `ports.ErrConflict` (defined
  once in `internal/ports`, returned by every adapter) →
  `connect.CodeNotFound` / `connect.CodeAlreadyExists`.
- Anything else → `connect.CodeInternal` with a generic client-facing
  message; the real error is `slog`'d server-side with `trace_id`/`span_id`
  per [0008](0008-structured-logging.md) — internal error detail never
  leaks to the client by default.

### Telemetry, logging, reflection

- `connectrpc.com/otelconnect` provides the tracing/metrics interceptor per
  [0007](0007-telemetry.md) — application code still only touches the OTel
  API; SDK/exporter wiring stays exclusively in `cmd/purser`.
- A `slog`-based interceptor logs every RPC (method, status code,
  duration) with a `component=api.connect` attribute per
  [0008](0008-structured-logging.md), correlated to the active span.
- `connectrpc.com/grpcreflect` is wired into `cmd/purser serve` so
  `grpcurl`/`buf curl` work against a running server for manual debugging.

### Testing

- Go-level: `internal/api/connect/*_test.go` follow [0003](0003-go-testing-standards.md)'s
  API-layer convention — routing/status/payload shape against a faked
  service, never a real one.
- k6 endpoint suites: **one file per entity in each of two protocols**,
  both required, neither optional — `test/k6/grpc/<entity>_test.js` calls
  every RPC the entity's service exposes natively via `k6/net/grpc`
  (loading the `.proto` directly, no server reflection dependency);
  `test/k6/http/<entity>_test.js` calls the same RPCs via Connect's
  HTTP/JSON transport with k6's built-in `http` module. Each file exercises
  the entity's full CRUD lifecycle plus `GetXDeletionImpact` where the
  entity has one, ending with a post-delete not-found check — see
  `test/k6/grpc/group_test.js` for the shape every entity's suite follows.
  These exercise the real running server end-to-end; they are not a
  substitute for the Go-level handler tests, which run without a live
  server.
- k6 flow suites (`test/k6/flow/`): a second, distinct category —
  multi-entity, scenario-shaped scripts modeling a real task a UI/admin
  tool would perform end-to-end (e.g. `create_studio_test.js`: register a
  Network, then a Studio under it, tag both, verify both tag-lookup
  directions, tear down in reverse order). One flow per meaningful
  cross-entity scenario, not one per entity and not one per RPC — most
  entities are already covered by their endpoint suite and need no flow
  test of their own. Each flow gets both a native-gRPC and an HTTP/JSON
  variant (`<name>_test.js` / `<name>_http_test.js`), same two-protocol
  requirement as endpoint suites.
- `make k6 endpoint` runs every `test/k6/grpc/*.js` and `test/k6/http/*.js`
  file; `make k6 flow` runs every `test/k6/flow/*.js` file; `make k6` with
  no argument runs both. All three require a running server (`make serve`
  or equivalent) — they are not run by `go test`.

## Consequences

- The web UI, k6, and any future consumer share exactly one generated
  contract — no drift between a "gRPC API" and a "REST API" maintained as
  two codebases.
- Connect's HTTP/JSON is RPC-shaped, not resource-oriented REST. Anyone
  expecting `GET /persons/123` later needs a deliberate new decision (e.g.
  `vanguard` or a hand-rolled REST facade), not an assumption that this ADR
  already provides it.
- SRP-per-entity means composed views (a performer's profile + credits +
  images) require an explicit, separate composing service later — this ADR
  deliberately does not build one yet, matching the "complete domain API
  without metadata providers" scope agreed for this pass.
- Pinned tools living outside `go.mod` means `go get`/`go mod tidy` can
  never accidentally drag buf's heavy dependency tree into the application
  binary's build — but it also means CI must run `make tools` explicitly
  before `make proto-gen`/`make k6`, rather than tools "just being there"
  from `go mod download`.
- Generated code is committed, so reviewing a proto change means reviewing
  a `gen/go/` diff too — larger PRs, but a build that never silently drifts
  from committed generated code.

## Self-Audit Checklist

1. Does any `internal/api/connect` handler call a port or adapter directly
   instead of going through `internal/service`? If yes — fix it.
2. Does any handler or service reach out to a metadata provider, or accept
   less than a fully-formed entity and try to fill in the gaps itself? If
   yes — that capability belongs in a separate, explicit RPC, not folded
   into a write path.
3. Does any `internal/service` type depend on more than one entity's port
   (a "God service")? If yes — split it.
4. Does any handler construct a `connect.NewError` inline instead of going
   through the shared error-mapping helper? If yes — fix it.
5. Does every new entity's service have a Go handler test (faked service)
   *and* both a `test/k6/grpc/<entity>_test.js` and
   `test/k6/http/<entity>_test.js` endpoint suite covering every RPC it
   exposes? If any are missing — add them before merging.
5a. Does this change introduce a real, cross-entity user task (not just a
    new entity's own CRUD) without a `test/k6/flow/` scenario covering it
    in both protocols? If yes — add one; if the change is just another
    entity's ordinary CRUD with no novel multi-entity interaction, a flow
    test is not required — don't add one speculatively.
6. Is anything under `gen/go/**` hand-edited instead of produced by
   `make proto-gen`? If yes — revert and fix the `.proto` instead.
7. Does any package outside `cmd/purser` import `otel/sdk/*`, a concrete
   exporter, or construct its own interceptor chain's provider? If yes —
   that wiring belongs in the composition root per [0007](0007-telemetry.md).
