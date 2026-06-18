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

## UI Architecture

The React UI is an API consumer with no privileged access. The same REST API that the UI uses must be sufficient for any other client.

- Shared UI patterns (cards, detail pages, search dialogs) must be built as reusable components, not duplicated per content type.
- Content-type-specific behavior is driven by props/config, not by forked component trees.
- A component built for one content type that would be needed identically in another must be extracted to `components/` before landing.
