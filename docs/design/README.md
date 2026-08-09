# Web UI Design Docs

These are living reference documents, not ADRs — they're expected to
iterate as the frontend actually gets built, unlike `docs/adr/`'s
one-time architectural decisions. When one of these docs and an ADR
disagree on something in scope for both (e.g. API access pattern), the
ADR wins; these docs exist to fill the gap ADRs don't cover: UX/IA and
visual design.

Read in this order — each builds on the one before it:

1. [ux-principles.md](ux-principles.md) — what Purser is, navigation/IA,
   state vocabulary, search, feedback, progressive disclosure,
   accessibility baseline. Read this first; it's the design tiebreaker
   for everything else.
2. [style-guide.md](style-guide.md) — the visual system: theming
   architecture, color, typography, spacing, motion, iconography,
   responsive breakpoints, component vocabulary.
3. [frontend-stack.md](frontend-stack.md) — the "configuration" layer:
   Connect-Web data fetching, routing, Tailwind, directory layout. The
   framework scaffold this describes is implemented (see its "What's
   built vs. what's next" section) — no module screens yet.

## Status

`ux-principles.md`/`style-guide.md` are **Draft** — reviewed once,
expected to take at least one more revision pass before being treated as
settled. `frontend-stack.md`'s scaffold is implemented and running
(Tailwind, routing, Connect-Web/Connect-Query, the Layout shell, and
Go-side embedding all wired end-to-end behind one Welcome page) — no
module (Music/AfterDark/Acquisition/Pipeline) screens exist yet.

## Related ADRs

- [0004](../adr/0004-typescript-react-testing-standards.md) — TS/React
  testing and shared-component reuse rules these docs build on.
- [0011](../adr/0011-api-design.md) — Connect-primary RPC API; the
  binding constraint behind `frontend-stack.md`'s data layer.
- [0023](../adr/0023-job-queue.md) / [0024](../adr/0024-pipeline-core.md)
  — the Job/Pipeline model behind `ux-principles.md`'s state vocabulary
  and Activity/Jobs surface.
