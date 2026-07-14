# Architecture Decision Records — Index

ADRs record *why* a decision was made, not just what the current code looks like.
They are the source of truth for architecture and process questions — when this
index or an individual ADR conflicts with older docs elsewhere in the repo, the
ADR wins.

## Rules for every session

1. **Before writing any code**, read the ADR(s) relevant to the change and state
   whether the planned approach conforms. If no ADR covers the area, say so —
   that gap is worth flagging and may justify a new ADR.
2. **After finishing any ask/task/session**, run the self-audit checklist embedded
   in [0001](0001-hexagonal-architecture.md) and [0002](0002-solid-design-principles.md)
   and report the result explicitly, even when the answer is "no violations found."
3. Do not treat ADRs as read-once background. Re-check them every time, every task.

## Current ADRs

| ADR | Title | Status |
|---|---|---|
| [0001](0001-hexagonal-architecture.md) | Hexagonal Architecture | Accepted |
| [0002](0002-solid-design-principles.md) | SOLID Design Principles | Accepted |
| [0003](0003-go-testing-standards.md) | Go Testing Standards | Accepted |
| [0004](0004-typescript-react-testing-standards.md) | TypeScript/React Testing Standards | Accepted |
| [0005](0005-github-issue-format.md) | GitHub Issue Format & Labels | Accepted |
| [0006](0006-commit-conventions.md) | Commit Message Conventions | Accepted |
| [0007](0007-telemetry.md) | Telemetry: OpenTelemetry Tracing and Metrics | Accepted |
| [0008](0008-structured-logging.md) | Structured Logging with slog | Accepted |
| [0009](0009-cli-stack.md) | CLI Stack: Cobra, pterm, and Bubble Tea | Accepted |
| [0010](0010-configuration.md) | Configuration: Viper | Accepted |
| [0011](0011-api-design.md) | API Design: Connect-Primary RPC API | Accepted |
| [0012](0012-datastore-persistence.md) | Persistence: A Generic Datastore Behind Badger and SQL | Accepted |
| [0013](0013-image-blob-storage.md) | Image Blob Storage: A Local-Filesystem-First `ImageStore` Port | Accepted |
| [0014](0014-search-embedded-full-text-index.md) | Search: An Embedded Full-Text Index (Bleve), Not a Search Server | Accepted |
| [0015](0015-deletion-impact-and-composing-services.md) | Deletion Impact and Composing Services | Accepted |
| [0016](0016-bulk-operations.md) | Bulk Operations: Batch at the Storage Layer, Bulk Endpoints Only Where Needed | Accepted |
| [0017](0017-build-and-release-goreleaser.md) | Build & Release: Goreleaser as the Build Model | Accepted |
| [0018](0018-local-development-environment.md) | Local Development Environment: Shared Directories, One Compose File, One Postgres | Accepted |
| [0019](0019-tag-identity-and-get-or-create.md) | Tag Identity: (Scope, Key, Value) Uniqueness via a Reservation Document | Accepted |
| [0020](0020-server-generated-kernel-entity-ids.md) | Server-Generated Kernel Entity IDs (UUIDv7) | Accepted |
| [0021](0021-music-domain-model.md) | Music Domain Model: Artist/Release Group/Release/Track on the Shared Kernel | Accepted |

## Adding a new ADR

Copy the template below into `docs/adr/NNNN-short-title.md`, using the next
sequential number. Add a row to the table above and link it from `AGENTS.md`
if it should be loaded for a specific task type.

```markdown
# NNNN. Title

Status: Proposed | Accepted | Superseded by NNNN

## Context
What situation forced this decision? What was tried before and failed?

## Decision
The rule, stated so it can be mechanically checked.

## Consequences
What this makes easier, what it makes harder, what it forbids outright.

## Self-Audit Checklist (if applicable)
Concrete yes/no questions to ask after writing code in this area.
```
