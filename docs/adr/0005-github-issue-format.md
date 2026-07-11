# 0005. GitHub Issue Format & Labels

Status: Accepted

## Context

Issues need a consistent shape so scope, priority, and area are visible
without reading the full description, and so an epic/task hierarchy doesn't
have to be reconstructed from memory every time. This ADR documents the
label taxonomy already in use in this repository (source of truth: `gh label
list`) and the required issue shape.

## Decision

### Label taxonomy

**Type** (what kind of change):
- `type: bug` — something isn't working
- `type: feature` — new functionality
- `type: chore` — maintenance and tooling
- `type: docs` — documentation

**Area** (what part of the system):
- `area: api` — REST API layer
- `area: adapter` — external system adapters
- `area: db` — database and migrations
- `area: ui` — React frontend
- `area: domain` — domain model and business logic
- `area: infra` — CI/CD, containers, deployment

**Scope** (issue hierarchy):
- `scope: epic` — top-level feature issue (user story)
- `scope: task` — implementation sub-task under an epic

**Status** (workflow state):
- `status: proposed` — idea proposed, not yet scoped or prioritized
- `status: ready` — ready to be worked on
- `status: in-progress` — currently being worked on
- `status: blocked` — blocked by another issue

**Priority:**
- `priority: high`
- `priority: low`
(absence of a priority label = normal/default priority)

**Other:**
- `breaking change` — breaking API or behavior change (label only; per
  breaking-change policy, do not use a `BREAKING CHANGE:` commit footer
  pre-1.0 — note it in the commit body instead)
- `good first issue`
- Automation-managed, do not hand-apply: `dependencies`, `github_actions`,
  `javascript`, `go`

Every issue gets exactly one `type:`, one `scope:`, and one `status:` label
at creation. `area:` labels may be multiple if the change genuinely spans
layers. `priority:` is optional.

### Issue shape

- **Title:** imperative, no type prefix in the title (the label carries
  that) — e.g. "Add track reordering to album detail," not "[Feature] ...".
- **Epic issues** (`scope: epic`): describe the user-facing outcome, list
  the task issues that implement it (as a checklist of issue links, filled
  in as tasks are created), and state what's explicitly out of scope.
- **Task issues** (`scope: task`): reference the parent epic, describe the
  concrete unit of work, and state which ADR(s) govern the area being
  touched (per [0001](0001-hexagonal-architecture.md) /
  [0002](0002-solid-design-principles.md) if architectural, or
  [0003](0003-go-testing-standards.md)/[0004](0004-typescript-react-testing-standards.md)
  if it's about test expectations).
- **Bug issues** (`type: bug`): reproduction steps, expected vs. actual,
  and which layer (domain/adapter/api/ui) the bug lives in if known.

## Consequences

- Labels are applied from a fixed, documented set — no ad hoc new labels
  invented per issue. If a genuinely new category is needed, that's an ADR
  update, not a silent new label.
- Every task issue is traceable to the ADR that constrains it, which is what
  makes "check the ADR before writing code" (see [0000-index](0000-index.md))
  actionable at the issue level, not just at the session level.

## Self-Audit Checklist

1. Does this issue have exactly one `type:`, one `scope:`, and one
   `status:` label?
2. If this is a `scope: task`, does it link its parent epic and name the
   governing ADR(s)?
3. Am I about to apply a label that doesn't exist in the taxonomy above? If
   yes — stop and use the closest existing label, or raise updating this
   ADR instead of inventing one.
