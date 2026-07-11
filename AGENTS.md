# !! HARD BAN !!

NEVER run `git commit`, `git push`, or `gh pr create`. Not ever. Not for any reason.
The user runs git themselves. Claude produces the commit message as text and stops.

---

# Purser — Agent Index

Do not read all project documentation. Determine what the task touches, then load only the required files listed below.

---

## Architecture Decision Records — read before writing any code

- [docs/adr/0000-index.md](docs/adr/0000-index.md) — ADR index, template, and the standing rules below

**Before writing any code:** read the ADR(s) relevant to the change and state whether the planned approach conforms.

- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md) — port/adapter/domain boundaries; required for any change touching `internal/domain`, `internal/ports`, `internal/adapters`, or `internal/service`
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md) — SOLID as applied in this codebase; required for any new service, port, or adapter
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md) — required for any Go test written or Go coverage question
- [docs/adr/0004-typescript-react-testing-standards.md](docs/adr/0004-typescript-react-testing-standards.md) — required for any `web/` component, page, or hook
- [docs/adr/0005-github-issue-format.md](docs/adr/0005-github-issue-format.md) — required before creating or labeling a GitHub issue
- [docs/adr/0006-commit-conventions.md](docs/adr/0006-commit-conventions.md) — required whenever producing commit message text (subject format, body style, `Closes`/`Part-Of` footers)
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md) — OpenTelemetry tracing/metrics; required for any code that instruments a request, cache, or adapter, or touches `cmd/`'s SDK/exporter wiring
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md) — slog usage; required for any code that logs anything
- [docs/adr/0009-cli-stack.md](docs/adr/0009-cli-stack.md) — Cobra/pterm/Bubble Tea; required for any `cmd/**` command or interactive output
- [docs/adr/0010-configuration.md](docs/adr/0010-configuration.md) — Viper configuration; required for any new configurable component or `internal/config` change

**After finishing any ask/task/session:** run the self-audit checklist in [0001](docs/adr/0001-hexagonal-architecture.md) and [0002](docs/adr/0002-solid-design-principles.md) (and [0003](docs/adr/0003-go-testing-standards.md)/[0004](docs/adr/0004-typescript-react-testing-standards.md) if tests were written) and state the result explicitly — do not skip this silently.

---

## Plan before you act

Before doing any non-trivial work, present a plan and get explicit permission before executing. Frame the plan in terms of:

1. **Scope** — exactly what changes and what doesn't.
2. **Hexagonal architecture** — which ports/adapters/domain boundaries are touched (see [0001](docs/adr/0001-hexagonal-architecture.md)).
3. **SOLID** — which principles apply and how the approach satisfies them (see [0002](docs/adr/0002-solid-design-principles.md)).

Do not go down an open-ended investigation or implementation spiral that wasn't part of the approved plan.

---

## Load by Task

Documentation under `docs/` (architecture overview, development guides, technical references, project vision) was reset and is being rebuilt incrementally, one area at a time. Until a given doc exists, treat its absence as a gap to flag, not something to silently reconstruct from scratch. This table will be filled in as each area is rebuilt:

### Starting a new feature or issue
- [docs/adr/0005-github-issue-format.md](docs/adr/0005-github-issue-format.md)

### Writing Go code
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md)
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md) — if the code makes a network call, caches anything, or otherwise merits tracing/metrics
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md) — if the code logs anything

### Building a shared/reusable package in `pkg/`
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md) — `pkg/**` targets 80%, same as adapters
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md)
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md)

### Writing a CLI command
- [docs/adr/0009-cli-stack.md](docs/adr/0009-cli-stack.md)

### Adding or changing configuration
- [docs/adr/0010-configuration.md](docs/adr/0010-configuration.md)

### Writing TypeScript or React
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0004-typescript-react-testing-standards.md](docs/adr/0004-typescript-react-testing-standards.md)

### Changing ports or adapters
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)

### Committing or creating a PR
- Hard ban above still applies: produce the message, do not run the command.
- [docs/adr/0006-commit-conventions.md](docs/adr/0006-commit-conventions.md)

### Creating or labeling a GitHub issue
- [docs/adr/0005-github-issue-format.md](docs/adr/0005-github-issue-format.md)

---

## Diagnostic Rules

### "X is not displayed" bugs
Start in the UI layer. The first tool call must be against a UI file — component, type, or API hook. Before opening any Go file:
1. Find the component that renders X and check whether it reads the relevant field at all.
2. Check that the field exists on the TypeScript type (`web/src/types/index.ts`).
3. Only if both are wired up correctly, then check the API response (curl or network tab).
4. Only if the API response is wrong, go into the Go handler or service.

"Nothing displayed" is a UI bug until the network response proves otherwise.

### API response / TypeScript type parity (code review step)
Every field the Go API returns must exist on the corresponding TypeScript interface in `web/src/types/index.ts`. See [0004](docs/adr/0004-typescript-react-testing-standards.md).
