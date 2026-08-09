# Web UI — UX Principles

Status: Draft — living document, not ADR-numbered. Revisit whenever a new
screen surfaces a pattern this doc doesn't cover; update it instead of
inventing a one-off.

This document is why the web UI behaves the way it does. It does not
describe visual design (see [style-guide.md](style-guide.md)) or the
technical stack (see [frontend-stack.md](frontend-stack.md)).

## North Star

Design the web UI as **a continuous media experience, not a collection of
pages.** A screen that reads like a database table with thumbnails glued on
has failed, even if every field is technically present and correct. Every
decision below exists to keep that from happening.

## What Purser actually is

Before borrowing patterns from any reference app, state what Purser is,
because it determines which parts of those apps apply:

- Purser is a **self-hosted personal library manager and acquisition
  tool** — you own everything cataloged in it. There is no subscription
  catalog, no algorithmic "recommended for you" feed, no content you don't
  already have or haven't asked to acquire.
- Grepping the actual API surface (`proto/purser/**`) confirms this: the
  modules are acquisition (indexer search, download), afterdark
  (browse/performer-profile), music (release/MusicBrainz/TheAudioDB/
  fanart.tv), pipeline (scan/organize/unmatched-file review), job, and the
  domain kernel (person/group/item/library-entry/media-file/tag/image).
  **There is no playback service anywhere in the API.** Purser catalogs and
  organizes media; it does not play it.
- `Item.Status` (`proto/purser/domain/v1/common.proto`) is the closest
  thing to a Netflix-style lifecycle, and it reads like Sonarr/Radarr, not
  a streaming client: `WANTED → GRABBED → DOWNLOADING → IMPORTED`, with
  `MISSING`/`SKIPPED` as exception states. This is a **library-acquisition
  vocabulary**, not a **playback vocabulary**.

**Consequence for reference weighting** (per your answers): Plex/Jellyfin's
"this is your stuff" library mental model is primary. Netflix and Apple TV
are mined only for specific, transferable patterns — row-based browsing,
hero treatment, transient-control conventions, content-forward metadata
display — never for their subscription-catalog/recommendation IA, which
doesn't apply here. Where Purser needs a *management/status* surface
(acquisition queue, pipeline scan progress, unmatched-file review), Plex's
and Jellyfin's own **admin dashboard/activity panels** are the closer
precedent than either app's consumer-facing browse screen — that surface
exists in this project's reference stack specifically because Purser's job
queue ([ADR 0023](../adr/0023-job-queue.md)) and pipeline
([ADR 0024](../adr/0024-pipeline-core.md)) are structurally the same
problem Sonarr/Radarr/Lidarr/Plex/Jellyfin all solve with an activity feed.

## Reference stack

| Reference | Used for | Not used for |
|---|---|---|
| [Nielsen Norman Group](https://www.nngroup.com/articles/ten-usability-heuristics/) | Navigation, search, feedback, error handling, progressive disclosure — *why* the UI behaves as it does | Visual style |
| Apple HIG (media playback: [WWDC22 session](https://developer.apple.com/videos/play/wwdc2022/10147/)) | Content-forward metadata display, transient-control philosophy, generalized to *browsing* since Purser has no player | Literal scrubber/PiP/subtitle-track guidance — not applicable, Purser doesn't play media |
| [Material 3](https://m3.material.io/foundations) | Component/interaction vocabulary (cards, dialogs, states, layered design tokens) | Visual aesthetic — not adopting Material's look |
| [Laws of UX](https://lawsofux.com/) | Interaction-decision tiebreakers (Jakob's, Fitts's, Hick's, Doherty, Aesthetic-Usability, Von Restorff) | — |
| WCAG 2.2 | Accessibility floor (keyboard, focus, contrast, target size, motion, captions) | — |
| Plex / Jellyfin | Primary IA: library-ownership browsing, dashboard/activity for background jobs, poster-wall + detail-page structure | — |
| Netflix / Apple TV | Visual polish only: hero treatment, row-based shelves, hover/transient controls | Recommendation feed, subscription-catalog framing |

## Navigation & information architecture

- **Persistent primary navigation** across modules (Library, Music,
  AfterDark, Acquisition, Pipeline/Jobs) — a fixed-width sidebar that
  never disappears above the `md` breakpoint (collapsing to an off-canvas
  drawer below it, per [frontend-stack.md](frontend-stack.md#styling-tailwind-v4)),
  per Jakob's Law: match the convention Plex/Jellyfin users already carry
  into this app, don't invent a new pattern for its own sake. The sidebar
  is the *only* fixed-width element on the page — see
  [style-guide.md](style-guide.md#fluid-by-default--no-centered-container)
  for why everything else is fluid.
- **Back means "where you logically came from," not "previous URL."**
  Drilling from a library grid into a detail view and back returns to the
  same scroll position and filter state — losing that state on Back is a
  known failure mode of "page-shaped" apps and is exactly what the North
  Star forbids.
- **Modal drill-down for editing, route-based navigation for browsing.**
  Viewing an entity (LibraryEntry, Person, Group detail) is a real route,
  bookmarkable and shareable. Editing an entity, accepting a match
  candidate, or reviewing an `UnmatchedFile` decision is a dialog/panel
  layered on the current route — it should never orphan the user on a
  route that only makes sense mid-edit.
- **Search lives in the primary nav, always reachable**, not buried per
  module — but results are scoped per-module by default (music search
  doesn't return AfterDark results) with an explicit way to broaden, per
  NN/g's recognition-over-recall heuristic: show the scope, don't make the
  user remember it.

## State vocabulary

Every state shown in the UI must trace to a real enum in the API — no
inventing a "watched/in-progress/favorite" playback vocabulary, since
Purser has none of that:

| Entity | States | Source |
|---|---|---|
| `Item` | Wanted → Grabbed → Downloading → Imported, or Missing / Skipped | `ItemStatus`, `common.proto` |
| `UnmatchedFile` | Pending → Matched or Dismissed | `UnmatchedFileStatus`, `unmatched_file.proto` |
| `Job` (scan/organize/etc.) | Pending → Running → Succeeded / Failed / Partial | `JobStatus`, `job.proto`, per [ADR 0023](../adr/0023-job-queue.md) |
| Download (acquisition) | Queued → Downloading → Paused → Completed / Failed | `DownloadState`, `download.proto` |
| `LibraryEntry` | Open string (`status`), plus `MonitorMode` (All/Future/None/Latest) | `library_entry.proto`, `common.proto` |

`LibraryEntry.status` and `content_type`/`kind` are deliberately open
strings in the domain model (new content types must not require a proto
regeneration) — the UI renders whatever string comes back rather than
switching on a fixed enum, and falls back to a generic "unknown status"
treatment for values it doesn't specifically style. This mirrors
[ADR 0001](../adr/0001-hexagonal-architecture.md)'s rule that
content-type knowledge never leaks into shared code: a shared `LibraryEntry`
card component takes status as a prop/config, it does not hardcode a
switch statement over known content types.

## Search & discoverability

- Per-module scoped search by default (NN/g: match system behavior to the
  mental model — a music search returning AfterDark results violates
  "match between system and real world").
- Zero-results states are never a blank page: state what was searched,
  suggest broadening scope or checking the pipeline's `UnmatchedFile` queue
  if the user expected something that hasn't been matched yet.
- Filtering/faceting (by tag, content type, status) is visible and
  recognized, not recalled from memory — surfaced as chips/controls, not
  a syntax the user has to learn (NN/g heuristic 6).

## Feedback & system status

- Every long-running operation (scan, download, organize, identify) is a
  `Job` the UI can poll or stream, per [ADR 0023](../adr/0023-job-queue.md)
  — the UI never leaves a user watching a spinner with no path to "what's
  actually happening." A dedicated Activity/Jobs surface (Plex/Jellyfin's
  dashboard precedent) shows Task/Step-level progress, not just a binary
  done/not-done.
- **Doherty Threshold**: interactions under ~400ms get no loading
  indicator at all — inserting one where none is needed reads as *slower*,
  not more informative. Anything slower gets an explicit, specific status,
  not a generic spinner (per Apple's content-forward principle: tell the
  user what's happening, not just that something is).
- Errors are actionable, not just visible: a failed `Job`/`Item`/download
  states *why* (mapped from the Connect error, per
  [ADR 0011](../adr/0011-api-design.md)'s structured error model) and what
  to do next, never a bare "Internal Error."

## Progressive disclosure

- A library card shows only what's needed to recognize and act on an item
  (title, primary image, status badge) — everything else (full metadata,
  external IDs, match-candidate scores, provider attribution) lives behind
  the detail view. Hick's Law: more visible choices on a card slows down
  every single scan of a grid.
- Detail views group information the way Plex/Jellyfin do (overview →
  metadata → files/tracks → related), not as a flat field dump of every
  proto field the entity has.
- Edit dialogs are the *only* place raw/technical fields (external IDs,
  hashes, `MonitorMode`) surface — matching the existing
  `components/edit/editors/` pattern already named in this project's prior
  art, not a new one.

## Keyboard & accessibility baseline (WCAG 2.2)

Stated as a floor for every screen, not an afterthought pass at the end:

- **Keyboard-operable** (2.1.1, Level A): every action reachable by mouse
  is reachable by keyboard, full stop.
- **Visible focus** (2.4.7 AA) that is never obscured by sticky headers or
  overlays (2.4.11 AA).
- **Contrast**: body text ≥ 4.5:1, large text ≥ 3:1 (1.4.3 AA); UI
  component/graphical-object contrast ≥ 3:1 against adjacent colors
  (1.4.11 AA) — binding on every color token in
  [style-guide.md](style-guide.md), not just body copy.
- **Target size** ≥ 24×24 CSS px minimum (2.5.8 AA) for anything
  clickable — directly informs card/button sizing in the style guide.
- **Reduced motion**: `prefers-reduced-motion` respected for any
  auto-playing hover preview or transition (AAA 2.3.3, treated as a floor
  here regardless of level, since media-hover-preview is exactly the kind
  of animation this criterion exists for).
- Non-text content gets a text alternative (1.1.1 A) — every poster/cover
  image has real alt text sourced from entity metadata, not a filename.

## Open questions (not resolved here — flag before building on them)

- Composed detail views (e.g. a full performer profile combining
  `Person` + credits + images) require a future composing service per
  [ADR 0011](../adr/0011-api-design.md) — until it exists, some detail
  pages assemble multiple RPC calls client-side. This doc doesn't
  prescribe a client-side composition pattern yet; resolve it when the
  first such page is built, not speculatively here.
- Global (cross-module) search is named above as an escape hatch from
  per-module scoping, but no RPC for it exists yet — flagged, not
  designed, until a search service exists to back it.
