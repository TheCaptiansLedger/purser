# 0008. Structured Logging with slog

Status: Accepted

## Context

Purser will run as a long-lived background service across many concurrent
adapters, caches, and background jobs. A plain-text log line with a level and
a message is not enough to answer "which cache instance evicted this key" or
"which provider call is this error from" without grepping and guessing.
"Structured logging" is easy to claim and easy to satisfy only halfway (a log
level plus a formatted message string still isn't structured) if it isn't
pinned down.

## Decision

- **`log/slog` is the only logging API used in application code.** No
  `fmt.Print*`, `log.Print*`/`log.Fatal*`, or third-party logging library
  (zap, logrus, zerolog, etc.) in `pkg/**`, `internal/**`, or `cmd/**`.
- **Structured means attributes, not just a level.** A log call that only
  varies its message string to convey what happened (e.g.
  `slog.Info("cache miss for key " + key)`) is a violation. The message is a
  short, constant, human-readable string; everything that varies goes in as
  attributes: `slog.Info("cache miss", "cache.name", name, "key", key)`.
- **Every log call carries a component identity.** Packages that emit logs
  accept (or default-construct) a `*slog.Logger` already scoped with
  identifying attributes for that component — at minimum a `component`
  attribute (e.g. `component=httpclient`, `component=cache.memory`) and,
  where applicable, an instance identifier (e.g. `cache.name`). This is done
  via `logger.With(...)` once at construction, not repeated at every call
  site.
- **Trace correlation:** whenever a log call has an active span in its
  context, it includes `trace_id` and `span_id` attributes pulled from
  `trace.SpanContextFromContext(ctx)` (see [0007](0007-telemetry.md)) so logs
  and traces can be cross-referenced.
- **Handler/format is a composition-root concern.** Library code accepts an
  injected `*slog.Logger` (defaulting to `slog.Default()` if none is given —
  see [0007](0007-telemetry.md)'s no-cost-to-opt-out principle) and never
  constructs its own handler or decides output format (JSON vs. text)
  itself. `cmd/` is the only place that calls `slog.New` with a concrete
  handler and calls `slog.SetDefault`.
- **Errors are values, not string concatenation.** Always
  `slog.Any("error", err)` (or the shorthand attribute form), never
  `fmt.Sprintf("...: %v", err)` folded into the message.

## Consequences

- Every log line is machine-queryable on structured fields, not just
  greppable text — this is what makes "logs, metrics, and traces for free"
  (per [0007](0007-telemetry.md)) actually true for any caller of a shared
  package.
- Slightly more ceremony at construction time (scoping a logger with
  `With(...)`) in exchange for zero ceremony at every call site afterward.
- A package that wants a different log format for local development doesn't
  get to decide that itself — it's a composition-root decision, keeping
  format consistent across the whole binary.

## Self-Audit Checklist

1. Does any file import `fmt` for logging, or call `log.Print*`/`log.Fatal*`,
   or import a third-party logging library? If yes — replace with `slog`.
2. Does any `slog` call vary only its message string instead of using
   attributes for the variable data? If yes — fix it.
3. Does every package-level logger carry at least a `component` attribute
   (and instance identifier, where applicable) set once via `With(...)`
   rather than repeated per call? If no — fix it.
4. Does any package outside `cmd/` construct its own `slog.Handler` or call
   `slog.SetDefault`? If yes — that belongs in the composition root.
5. Is `err` ever folded into the message string via `fmt.Sprintf`/string
   concatenation instead of passed as a structured attribute? If yes — fix
   it.
