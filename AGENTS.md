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
- [docs/adr/0011-api-design.md](docs/adr/0011-api-design.md) — Connect-primary RPC API design; required for any `proto/**`, `internal/api/connect`, or `internal/service`/`internal/ports` change
- [docs/adr/0012-datastore-persistence.md](docs/adr/0012-datastore-persistence.md) — generic `Datastore` behind Badger/SQL; required for any `internal/adapters/datastore` or `internal/adapters/store` change
- [docs/adr/0013-image-blob-storage.md](docs/adr/0013-image-blob-storage.md) — `ImageStore` port and local-filesystem adapter; required for any `internal/adapters/imagestore` change or anything writing/reading image bytes
- [docs/adr/0014-search-embedded-full-text-index.md](docs/adr/0014-search-embedded-full-text-index.md) — `SearchIndex` port and the embedded-Bleve decision; required for any `internal/adapters/searchindex` change or anything indexing/querying free-text search
- [docs/adr/0015-deletion-impact-and-composing-services.md](docs/adr/0015-deletion-impact-and-composing-services.md) — `DeletionImpact`/Unlink-Cascade pattern and the composing-service exception to "one port per service"; required for any `Delete` flow that can leave other entities dangling, or any new composing service
- [docs/adr/0016-bulk-operations.md](docs/adr/0016-bulk-operations.md) — batch writes at the `Datastore` layer and when a bulk API endpoint is justified; required for any batch/multi-row operation
- [docs/adr/0017-build-and-release-goreleaser.md](docs/adr/0017-build-and-release-goreleaser.md) — goreleaser-driven binary/container builds; required for any `.goreleaser.yaml`, `ops/Containerfile`, `internal/version`, or release-pipeline (`.github/workflows/release.yml`) change
- [docs/adr/0018-local-development-environment.md](docs/adr/0018-local-development-environment.md) — shared `.local/` dev-state layout, single `ops/compose.yml`, one Postgres with two roles; required for any `ops/compose.yml`, `ops/postgres/**`, `Makefile` compose target, or local dev-state directory change
- [docs/adr/0019-tag-identity-and-get-or-create.md](docs/adr/0019-tag-identity-and-get-or-create.md) — Tag's `(Scope, Key, Value)` uniqueness and the reservation-document pattern; required for any `internal/adapters/store/tag` change, any `TagService`/`TagDeletionService` change, or any new entity that needs a uniqueness constraint beyond its own `(collection, id)` primary key
- [docs/adr/0020-server-generated-kernel-entity-ids.md](docs/adr/0020-server-generated-kernel-entity-ids.md) — server-generated UUIDv7 `ID`s via `domain.NewID()`; required for any kernel entity's `Create` flow, any new single-ID kernel entity, or any change to `internal/domain/id.go`
- [docs/adr/0021-music-domain-model.md](docs/adr/0021-music-domain-model.md) — Music's Artist/Release Group/Release/Track model on the shared kernel; required for any `internal/domain/music`, `internal/adapters/store/music`, Music `internal/service`/`internal/ports` code, or `proto/purser/music/v1` change

**After finishing any ask/task/session:** run the self-audit checklist in [0001](docs/adr/0001-hexagonal-architecture.md) and [0002](docs/adr/0002-solid-design-principles.md) (and [0003](docs/adr/0003-go-testing-standards.md)/[0004](docs/adr/0004-typescript-react-testing-standards.md) if tests were written) and state the result explicitly — do not skip this silently.

---

## Plan before you act

Before doing any non-trivial work, present a plan and get explicit permission before executing. Frame the plan in terms of:

1. **Scope** — exactly what changes and what doesn't.
2. **Hexagonal architecture** — which ports/adapters/domain boundaries are touched (see [0001](docs/adr/0001-hexagonal-architecture.md)).
3. **SOLID** — which principles apply and how the approach satisfies them (see [0002](docs/adr/0002-solid-design-principles.md)).

Do not go down an open-ended investigation or implementation spiral that wasn't part of the approved plan.

---

## Git workflow

Once a plan is approved and before writing any code:

1. **If the work is tracked by a GitHub issue, assign it** to the authenticated `gh` user (`gh issue edit <N> --add-assignee @me`, or the equivalent explicit login) before starting implementation — not after.
2. **Move the issue's `status:` label to `status: in-progress`**, removing whatever `status:` label it currently carries — every issue gets exactly one, per [0005](docs/adr/0005-github-issue-format.md) (`gh issue edit <N> --remove-label "status: <old>" --add-label "status: in-progress"`).
3. **Post a short comment on the issue** noting work has started (e.g. "Starting to work on this issue.") — `gh issue comment <N> --body "..."`.
4. **Create a topic branch off the current branch** and switch to it before making any edits — never accumulate work directly on `develop`/`main`. Name it `<type>/<issue#>-<short-slug>` (e.g. `feat/447-server-generated-kernel-entity-ids`), matching the `type` from [0006](docs/adr/0006-commit-conventions.md) and the issue number if one exists. If a branch already exists for the issue, switch to it instead of creating a new one.

At the end of every task that produced a diff, whether or not it was asked for explicitly:

5. **Produce a commit message as text and stop** — per the hard ban above, never run `git commit`. Follow [0006](docs/adr/0006-commit-conventions.md) exactly: type/scope/breaking-marker, imperative-present summary under 80 characters, prose body explaining why, and a `Closes:`/`Part-Of:` footer if a GitHub issue is being tracked.

Do not wait to be asked for any of these steps — they are part of finishing the task, not a separate follow-up request.

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

### Building the persistence layer (`internal/adapters/datastore`, `internal/adapters/store`)
- [docs/adr/0012-datastore-persistence.md](docs/adr/0012-datastore-persistence.md)
- [docs/adr/0019-tag-identity-and-get-or-create.md](docs/adr/0019-tag-identity-and-get-or-create.md) — if the change touches `internal/adapters/store/tag`, or needs a uniqueness constraint beyond an entity's own `(collection, id)` primary key
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md)
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md)
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md)

### Building image blob storage (`internal/adapters/imagestore`)
- [docs/adr/0013-image-blob-storage.md](docs/adr/0013-image-blob-storage.md)
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md)
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md)
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md)

### Building a proto/Connect (gRPC/HTTP) API service or handler
- [docs/adr/0011-api-design.md](docs/adr/0011-api-design.md)
- [docs/adr/0020-server-generated-kernel-entity-ids.md](docs/adr/0020-server-generated-kernel-entity-ids.md) — if the change touches a kernel entity's `Create` flow
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md)
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md)
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md)

### Building search (`internal/adapters/searchindex`)
- [docs/adr/0014-search-embedded-full-text-index.md](docs/adr/0014-search-embedded-full-text-index.md)
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md)
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md)
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md)

### Building a delete flow or a composing service
- [docs/adr/0015-deletion-impact-and-composing-services.md](docs/adr/0015-deletion-impact-and-composing-services.md)
- [docs/adr/0011-api-design.md](docs/adr/0011-api-design.md)
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)

### Building the Music module (`internal/domain/music`, `internal/adapters/store/music`, `proto/purser/music/v1`)
- [docs/adr/0021-music-domain-model.md](docs/adr/0021-music-domain-model.md)
- [docs/adr/0012-datastore-persistence.md](docs/adr/0012-datastore-persistence.md)
- [docs/adr/0015-deletion-impact-and-composing-services.md](docs/adr/0015-deletion-impact-and-composing-services.md) — the Music release deletion service, and the required `GroupDeletionService`/`LibraryEntryDeletionService` referrer updates
- [docs/adr/0011-api-design.md](docs/adr/0011-api-design.md)
- [docs/adr/0020-server-generated-kernel-entity-ids.md](docs/adr/0020-server-generated-kernel-entity-ids.md)
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)
- [docs/adr/0002-solid-design-principles.md](docs/adr/0002-solid-design-principles.md)
- [docs/adr/0003-go-testing-standards.md](docs/adr/0003-go-testing-standards.md)
- [docs/adr/0007-telemetry.md](docs/adr/0007-telemetry.md)
- [docs/adr/0008-structured-logging.md](docs/adr/0008-structured-logging.md)

### Building a bulk/batch operation
- [docs/adr/0016-bulk-operations.md](docs/adr/0016-bulk-operations.md)
- [docs/adr/0012-datastore-persistence.md](docs/adr/0012-datastore-persistence.md)
- [docs/adr/0015-deletion-impact-and-composing-services.md](docs/adr/0015-deletion-impact-and-composing-services.md) — if the batch operation is a delete
- [docs/adr/0001-hexagonal-architecture.md](docs/adr/0001-hexagonal-architecture.md)

### Building or releasing binaries/containers (goreleaser, `ops/Containerfile`, `.github/workflows/release.yml`)
- [docs/adr/0017-build-and-release-goreleaser.md](docs/adr/0017-build-and-release-goreleaser.md)
- [docs/adr/0009-cli-stack.md](docs/adr/0009-cli-stack.md) — if the change touches `internal/version`/`cmd/purser` version wiring

### Changing local dev environment (`ops/compose.yml`, `ops/postgres/**`, `.local/` layout, `make compose`)
- [docs/adr/0018-local-development-environment.md](docs/adr/0018-local-development-environment.md)
- [docs/adr/0010-configuration.md](docs/adr/0010-configuration.md)
- [docs/adr/0012-datastore-persistence.md](docs/adr/0012-datastore-persistence.md) — if it touches Postgres as a `database.driver` option

### Changing k6 tests, k6 CI wiring, or Makefile k6 targets (`test/k6/**`, `.github/workflows/pr.yml`'s `k6` job)
- [docs/adr/0022-k6-ci-enforcement.md](docs/adr/0022-k6-ci-enforcement.md)
- [docs/adr/0020-server-generated-kernel-entity-ids.md](docs/adr/0020-server-generated-kernel-entity-ids.md) — every k6 fixture that creates a kernel entity must read the id back from the Create response, never send one
- [docs/adr/0018-local-development-environment.md](docs/adr/0018-local-development-environment.md) — `.local/` dev-state convention for any new k6-CI-local directory

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
