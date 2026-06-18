# Go Development Standards

## Code Conventions

- No comments explaining what code does — names do that
- Comments only for non-obvious WHY: a hidden constraint, a workaround, a subtle invariant
- No global state — everything injected via constructor
- All time values in UTC; never `time.Local`

## Error Handling

Wrap errors with context at each layer boundary:

```go
return fmt.Errorf("context: %w", err)
```

## Context

Propagate `context.Context` everywhere. Never use `context.Background()` inside business logic — it belongs only at outermost entry points (main, test setup).

## Database Conventions

- `metadata JSON` columns hold type-specific fields that don't need indexed querying
- Fields that need filtering (performers, tags, quality) always get their own columns or junction tables

## Architecture Compliance

Go code must comply with the hexagonal architecture rules in `docs/architecture/overview.md` and the SOLID principles in `docs/architecture/principles.md`. Read both before writing any Go code.

## Linting

Run `golangci-lint run` before committing. All lint issues must be resolved — no suppressions without a comment explaining the exception.
