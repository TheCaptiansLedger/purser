# Architecture Principles

## Hexagonal Architecture (Enforce)

The dependency rule is absolute:

```
domain ← ports ← app ← adapters / api
```

Violations to watch for:

- An adapter importing from another adapter (cross-adapter coupling)
- An API handler containing business logic (should be in `app/`)
- A domain type importing anything from outside `domain/`
- Content-type-specific logic in `app/` instead of being driven by the domain model

## SOLID in Practice

**Single Responsibility**: each file/type does one thing. API handlers translate HTTP ↔ app service. App services orchestrate domain logic. Adapters translate external systems ↔ ports.

**Open/Closed**: new content types, metadata sources, and download clients are added by implementing a port — not by editing existing switch statements throughout the codebase.

**Interface Segregation**: ports are narrow. A metadata source adapter implements `MetadataSource`; it does not also implement repository methods.

**Dependency Inversion**: app services depend on port interfaces, never on concrete adapters.

## No Snowflakes — All Modules Are Identical (Hard Rule)

Every content type (music, tv, movie, adult, jav, book, and any future type) goes through the same code paths. No module is special. No module gets a private branch in the service or aggregator layer.

**Hard bans — these patterns are never acceptable:**

- A `switch`, `if`, or `case` in `internal/app/` or `web/src/components/` that compares against a content type name, entity kind name, or adapter/source name string.
- Selecting a specific adapter by name string (e.g. `sourceByName("fanart")`) in `internal/app/` to route logic.
- Hardcoding a priority list, role list, or capability list keyed by source or content type name in the service or aggregator.
- A shared UI component that branches on `contentType` string to change its behaviour or vocabulary — props/config drive that, not inline switches.

**The correct pattern when behaviour differs across content types:**

| Need | Wrong | Right |
|---|---|---|
| Different refresh job per entity kind | `if kind == KindArtist` in service | `kind.RefreshJobName()` declared on domain type |
| Only some kinds support a relationship | `if kind == KindArtist` guard | `kind.SupportsMemberRelationships()` on domain type |
| Source A has better images than source B | `priority := []string{"theaudiodb","fanart"}` | `ImagePriority() int` on the port; adapters declare their own rank |
| Group images require parent + child IDs | content-type branch in `FetchImagesForGroup` | `FindGroupImages(ctx, ct, parentExtID, groupExtID)` on the port |
| Role vocabulary per content type | switch in shared UI component | `GET /api/config/content-types` returns roles; component receives them as props |
| Adapter only handles one content type | nothing — this is normal | adapter's `ContentTypes()` declares scope; service fans out uniformly; adapter returns `ErrNotSupported` |

**The litmus test:** If a new content type were wired to a no-op adapter (all methods return `ErrNotSupported`) tomorrow, would any file in `internal/app/`, `internal/api/`, or `web/src/components/` need to change? If yes, there is a violation — fix the port instead.

## End-of-Implementation Checklist

Before closing any issue that touches `internal/app/`, `internal/adapters/`, or `web/src/components/`, verify:

1. Zero `domain.ContentType*`, `domain.Kind*`, or `domain.Source*` constants used for routing in `internal/app/`.
2. Zero adapter name string literals used for selection in `internal/app/`.
3. No shared `web/src/components/` file switches on content type, kind, or module name.
4. A new content type wired to a no-op adapter compiles and runs without errors and without editing any existing file outside the adapter and `cmd/`.
5. Every new capability on `MetadataSource` returns `ErrNotSupported` in all existing adapters that do not implement it — no silent omissions.

## UI Architecture

The React UI is an API consumer with no privileged access. The same REST API that the UI uses must be sufficient for any other client.

- Shared UI patterns (cards, detail pages, search dialogs) must be built as reusable components, not duplicated per content type.
- Content-type-specific behavior is driven by props/config, not by forked component trees.
- A component built for one content type that would be needed identically in another must be extracted to `components/` before landing.
