# 0001. Hexagonal Architecture

Status: Accepted

## Context

Purser supports multiple content types (music, movies, TV, books, adult/JAV,
and future types) each backed by different external metadata sources and
different storage needs. Prior implementations repeatedly collapsed this
variation into the service layer as `if contentType == "music"` branches and
direct calls to specific adapters by name, which made every new content type
an edit to existing, already-working code, and made adapters impossible to
test or swap in isolation.

## Decision

The codebase is organized in three layers, and dependencies only point inward:

```
adapters  --->  ports  <---  domain / services
(driven)       (interfaces)      (driving)
```

- **Domain** (`internal/domain/...`): plain Go types and business rules. No
  imports of adapters, no imports of any external SDK/HTTP client, no
  knowledge of specific content types beyond shared shape (title, tags,
  people, external IDs, etc.).
- **Ports** (`internal/ports/...`): interfaces the domain/service layer depends
  on — e.g. `MetadataSource`, `ImageSource`, `Repository`. Ports describe
  *capability*, never a specific provider. A port must be satisfiable by a
  fake/mock with zero knowledge of any real adapter.
- **Adapters** (`internal/adapters/...`): concrete implementations of ports —
  MusicBrainz, TheAudioDB, Cover Art Archive, Last.fm, BadgerDB, SQL, HTTP
  handlers, etc. Adapters may know about content types and provider-specific
  quirks. That knowledge must never leak upward into ports or domain.
- **Services** (`internal/service/...` or equivalent): orchestrate domain +
  ports. A service must be constructible with any conforming implementation
  of its dependency ports, including a test fake, without modification.

Rules that follow directly from this:

1. A service or domain type may depend on a port interface. It may never
   import a specific adapter package directly.
2. An adapter that cannot support a capability returns a typed
   `ErrNotSupported` (or equivalent) rather than being silently omitted from a
   type switch in the caller. Callers check the error, not the adapter's
   identity.
3. Content-type-specific behavior (role vocabulary, source priority, refresh
   cadence, etc.) lives in adapter implementations or configuration data,
   never in service-layer conditionals.
4. Adding a new content type must be possible by adding new adapters and
   configuration — zero edits to existing service-layer code. If it isn't,
   the design is wrong, not the new content type.

## Consequences

- Adapters are swappable and independently testable behind a fake port
  implementation — no live network calls needed to test service logic.
- Adding a content type is additive, not a modification of shared code
  (Open/Closed — see [0002](0002-solid-design-principles.md)).
- It costs more upfront design effort per feature: every new capability must
  be expressed as a port before it can be used, rather than reached for
  directly.

## Self-Audit Checklist

Run after every task that touches domain, service, or adapter code:

1. Does any code branch on a content-type name or module name (e.g.
   `if kind == "music"`)? If yes — fix it, push the behavior into an
   adapter or config.
2. Does any code reach for a specific adapter/source by name string instead
   of going through a declared port interface? If yes — fix it.
3. Does any code put domain knowledge (source priority, role vocabulary,
   refresh strategy) in the service/aggregator layer instead of in the
   adapter or configuration? If yes — fix it.
4. Does the service layer know about adapter internals (types, error codes,
   response shapes) beyond what the port interface exposes? If yes — fix it.
5. Would this code work correctly if a new content type were added tomorrow
   with zero changes to it? If no — the design is wrong.
