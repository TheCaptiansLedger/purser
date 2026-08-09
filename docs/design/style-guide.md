# Web UI — Style Guide

Status: Draft — living document, not ADR-numbered. This is the visual
system; see [ux-principles.md](ux-principles.md) for interaction/IA
decisions this builds on, and [frontend-stack.md](frontend-stack.md) for
how tokens get implemented in code.

## Theming architecture

**Ship dark-only. Architect for more.** Per your decision: only a dark
theme is designed and built right now, but every color is a **semantic
token**, never a literal hex value used directly in a component. That's
the difference between "dark-only" and "dark-only, permanently" — adding a
light theme, a high-contrast theme, or letting a user pick an accent color
later is a matter of defining a second token set, not rewriting
components.

Two token layers (Material 3's reference/system layering, adapted — see
[Reference stack](ux-principles.md#reference-stack)):

1. **Palette tokens** — raw values, never referenced directly by
   components: `--palette-ink-950: #08080e`, `--palette-violet-500: ...`,
   etc.
2. **Semantic tokens** — what components actually use:
   `--color-bg`, `--color-surface`, `--color-surface-raised`,
   `--color-border`, `--color-text`, `--color-text-secondary`,
   `--color-text-muted`, one `--color-accent-<content-type>` per content
   type plus `--color-accent-system` for cross-content-type surfaces, and
   the status palette (`--color-status-*`, aliased as `--color-danger`/
   `--color-success`/`--color-warning`) — all defined with real values
   below. A future theme redefines the semantic layer against a different
   palette; component code never changes.

Implemented as Tailwind v4 `@theme` custom properties (see
[frontend-stack.md](frontend-stack.md)) so both Tailwind utility classes
and raw CSS resolve the same tokens.

## Color

Every value below is verified, not assumed — contrast ratios are
computed with the WCAG relative-luminance formula against the actual
neutral tokens they're paired with, not eyeballed from the pre-reset
hexes they mostly (but not entirely) carry forward.

### Neutral scale

Starting point is this project's own prior art (`web/src/index.css`,
pre-reset):

| Token | Value | Use |
|---|---|---|
| `--color-bg` | `#08080e` | App background |
| `--color-surface` | `#0f0f1a` | Cards, panels |
| `--color-surface-raised` | `#161628` | Modals, popovers, hover state |
| `--color-border` | `rgba(255,255,255,0.06)` | Default dividers |
| `--color-border-hover` | `rgba(255,255,255,0.14)` | Interactive-element border on hover/focus |
| `--color-text` | `#f0f0f8` | Primary text |
| `--color-text-secondary` | `#8080a0` | Secondary text, metadata |
| `--color-text-muted` | `#50506a` | Disabled/placeholder only — see restriction below |

Computed body-text contrast (WCAG 1.4.3, AA needs ≥4.5:1):

| Pair | Ratio | Passes AA? |
|---|---|---|
| `--color-text` on `--color-bg` | 17.62:1 | Yes |
| `--color-text` on `--color-surface` | 16.79:1 | Yes |
| `--color-text` on `--color-surface-raised` | 15.71:1 | Yes |
| `--color-text-secondary` on `--color-bg` | 5.24:1 | Yes |
| `--color-text-secondary` on `--color-surface` | 5.00:1 | Yes |
| `--color-text-secondary` on `--color-surface-raised` | 4.67:1 | Yes |
| `--color-text-muted` on any surface | 2.29–2.56:1 | **No** |

**`--color-text-muted` fails AA outright and must stay restricted to
disabled/placeholder UI, never used for text that conveys real
information.** WCAG's contrast criteria don't apply to inactive controls,
which is the only reason this token is usable at all — treat that as a
hard boundary on where it appears, not an oversight to eventually fix.

### Content-type accents

One hue per `content_type` (open vocabulary — see
[ux-principles.md](ux-principles.md#state-vocabulary)) — used for nav
icons, hero-backdrop tint, and card borders. **Never used as body text**;
verified only against the 3:1 non-text-contrast floor (WCAG 1.4.11), not
the 4.5:1 text floor.

| Content type | Token | Hex | vs. surface | vs. bg |
|---|---|---|---|---|
| `music` | `--color-accent-music` | `#10b981` | 7.50:1 | 7.87:1 |
| `afterdark` | `--color-accent-afterdark` | `#f43f5e` | 5.18:1 | 5.44:1 |

Both are the exact pre-reset values (`web/src/config/modules.ts`) — kept
as-is since they already clear WCAG by a wide margin and are real,
previously-shipped prior art, not invented fresh. Adding a third content
type later: pick the next unused Tailwind 500-level hue, verify it here
before use, extend this table — never hardcode a switch over a fixed set
of content types (that's exactly what
[ux-principles.md](ux-principles.md#state-vocabulary) already forbids for
`LibraryEntry.status`/`content_type`).

**Acquisition and Pipeline/Jobs intentionally get no accent of their
own** — they're cross-content-type operational surfaces (a download or a
scan job can belong to any content type), not a content collection, so
they use the neutral/system accent below instead of competing with
whichever content type's items happen to be passing through them:

| Surface | Token | Hex | vs. surface |
|---|---|---|---|
| System/neutral (Library umbrella, Acquisition, Pipeline/Jobs, People, Tags, Settings) | `--color-accent-system` | `#6366f1` | 4.26:1 |

### Status/semantic tokens

A single shared 7-bucket palette that **every** state-bearing entity
maps onto, instead of four independent color systems. Rendered the same
way the pre-reset `ItemStatusBadge` shipped it: badge text is the full
color, badge background is that same color at ~13% alpha over
`--color-surface`.

| Bucket | Token | Hex | Text-on-own-tint contrast |
|---|---|---|---|
| Pending / not started | `--color-status-pending` | `#60a5fa` | 6.16:1 |
| Queued / next-up | `--color-status-queued` | `#a78bfa` | 5.78:1 |
| Active / in progress | `--color-status-active` | `#818cf8` | 5.34:1 |
| Success | `--color-status-success` (= `--color-success`) | `#34d399` | 7.91:1 |
| Warning / partial / paused | `--color-status-warning` (= `--color-warning`) | `#fbbf24` | 8.93:1 |
| Failure | `--color-status-failure` (= `--color-danger`) | `#f87171` | 5.79:1 |
| Neutral / skipped / dismissed | `--color-status-neutral` | `#9ca3af` | 6.14:1 |

Every bucket clears 4.5:1 as badge text by a comfortable margin. Six of
these seven hexes are the pre-reset `ItemStatusBadge`'s exact values;
`queued` is the one addition, needed because `Item.Status` already has a
third "in-between" step (`Grabbed`, between not-yet-started and actively
transferring) that a 2-bucket pending/active split can't express — and
reusing it lets `Download`'s `Paused` land on `warning` rather than
inventing an eighth bucket.

`--color-danger`/`--color-success`/`--color-warning` (referenced earlier
in this doc under Iconography/Motion) are aliases of the `failure`/
`success`/`warning` buckets — a form-validation error and a `Job`'s
"Failed" badge are deliberately the same visual signal, not two
unrelated reds.

**Entity → bucket mapping** — kept in sync with
[ux-principles.md](ux-principles.md#state-vocabulary)'s state table:

| Entity | pending | queued | active | success | warning | failure | neutral |
|---|---|---|---|---|---|---|---|
| `Item` | Wanted | Grabbed | Downloading | Imported | — | Missing | Skipped |
| `Job` | Pending | — | Running | Succeeded | Partial | Failed | — |
| `Download` | — | Queued | Downloading | Completed | Paused | Failed | — |
| `UnmatchedFile` | Pending | — | — | Matched | — | — | Dismissed |

## Typography

- **Inter Variable** (prior art), `system-ui`/`-apple-system` fallback —
  a single variable font covers the full weight range without multiple
  font-file requests, and it's already the established choice.
- Type scale follows Material 3's role-based naming
  (display/headline/title/body/label) rather than raw `h1`–`h6` sizing, so
  a component's text role is legible from its class name
  (`text-title-md`, not `text-2xl`) — this keeps the scale semantic the
  same way color is.
- Line-length and line-height tuned for scanning a grid of metadata-dense
  cards, not prose reading — short measure, tight leading on titles,
  slightly looser on body/overview text.

### Scale

Not invented from scratch — grounded in a frequency count of every
`text-*`/`font-*`/`leading-*`/`tracking-*` class actually used across the
pre-reset app's ~140 `.tsx` files. That data is unambiguous: this is a
small-text, metadata-dense app, not a headline-heavy one — `text-sm` and
`text-xs` alone are 93% of all size usage; `text-4xl`/`5xl` (hero titles)
account for 4 occurrences total. The scale below names Tailwind's
existing steps, it doesn't invent new ones — same rule as color and
spacing.

| Role | Token | Size / leading | Weight | Tracking | Real precedent |
|---|---|---|---|---|---|
| Display *(reserved, rare)* | `text-display` | `text-5xl` (3rem) / `leading-tight` | bold | normal | 1 occurrence — empty-library splash only, not a general heading |
| Headline | `text-headline` | `text-4xl` (2.25rem) / `leading-tight` | bold | normal | Hero/page titles |
| Title (large) | `text-title-lg` | `text-2xl` (1.5rem) / `leading-snug` | semibold | normal | Section headers, dialog titles — 13 occurrences |
| Title (medium) | `text-title-md` | `text-lg` (1.125rem) / `leading-snug` | semibold | normal | Card titles, sub-headers |
| Body (large) | `text-body-lg` | `text-base` (1rem) / `leading-relaxed` | normal | normal | Occasional emphasis paragraph — 8 occurrences |
| Body | `text-body` | `text-sm` (0.875rem) / `leading-relaxed` for prose, `leading-normal` otherwise | normal/medium | normal | **The default reading size** — 275 occurrences, the most-used class in the app by a wide margin |
| Label | `text-label` | `text-xs` (0.75rem) / `leading-normal` | medium | normal | Metadata, chips, table cells — 245 occurrences |
| Label (eyebrow) | `text-label-eyebrow` | `text-xs` (0.75rem) / `leading-none` | semibold | `tracking-widest`, uppercase | Section labels, breadcrumb-style links (e.g. a `DISCOGRAPHY` section header) — 64 occurrences of this exact combination, always paired with `--color-text-secondary`/`--color-text-muted` |

**`label` is restricted to secondary/tertiary chrome — badges,
timestamps, table cells, eyebrow text — never primary readable content.**
At 12px it's the typography-side twin of
[`--color-text-muted`'s restriction](#neutral-scale): heavily used, but
only where being small is the point.

**Sizing is always `rem`-based (Tailwind's default unit), never a fixed
`px` override in component code** — this is what makes WCAG 1.4.4
(Resize Text, AA) hold: 200% browser zoom must work without breaking
layout, which only happens if font sizes scale with the root, not a
hardcoded pixel value.

**WCAG 1.4.12 (Text Spacing, AA) check**: `body`/`body-lg` use
`leading-relaxed` (≈1.625×) or `leading-normal` (≈1.5×) — both clear the
1.5× line-height floor the criterion requires for paragraph text.
`headline`/`title-*`/`label-eyebrow` use tight/snug/none leading, which
is fine — these are short, effectively single-line strings, so 1.4.12's
overlap risk (which is about multi-line text) doesn't apply to them.

## Spacing & layout

- 4px base unit, standard Tailwind spacing scale (`4, 8, 12, 16, 24, 32,
  48, 64...`) — no custom scale invented, per Jakob's Law applied to
  *developers*: an unmodified Tailwind scale is legible to anyone who's
  used Tailwind before.
- Grid gutters and card padding scale with breakpoint (tighter on mobile,
  more breathing room on desktop/4K) rather than a fixed pixel value that
  reads as cramped or wasteful at the extremes.

### Named steps

Same rule as typography: the scale isn't reinvented, but *which* of
Tailwind's default steps this app actually reaches for is named here,
grounded in the same frequency count run across the pre-reset `.tsx`
files (`gap-*`/`p*-*` usage):

| Name | Step | Px | Real use |
|---|---|---|---|
| Micro | `1` | 4px | Icon-to-label gap inside a single control |
| Tight | `2` | 8px | Chip/button internal padding, inline clusters — `gap-1`/`gap-2` combined are the single largest bucket in the app |
| Compact | `3` | 12px | Default inter-element gap in dense lists/toolbars; button horizontal padding |
| **Base** | `4` | 16px | **The default** — card padding, grid gutter, standard component spacing |
| Comfortable | `6` | 24px | Section-level gaps, vertical rhythm between blocks |
| Spacious | `8` | 32px | Page-level horizontal padding at the widest breakpoints, tightened at narrower ones per the [fluid-by-default rule](#fluid-by-default--no-centered-container) below |
| Section | `10` | 40px | Major vertical rhythm between page sections |

### Corner radius

| Name | Token | Px | Real use |
|---|---|---|---|
| Small | `rounded-md` | 6px | Small chips, inputs |
| **Default** | `rounded-lg` | 8px | Cards, buttons, inputs — the clear workhorse, used far more than any other radius |
| Large | `rounded-xl` | 12px | Larger surfaces — hero images, modals |
| Pill | `rounded-full` | — | Avatars, circular icon buttons, status dots |
| Oversized *(rare)* | `rounded-2xl` | 16px | Large hero/backdrop treatments only |

### Fluid by default — no centered container

**The page shell and every grid/hero are fluid. No `max-w-* mx-auto`
wrapper around page content, ever.** This has already been gotten wrong
once and fixed once, in this project's own history: pre-reset commit
`24cabe5` (`feat(ui): ensure fluid design for all pages`, closing #305)
exists specifically because centered fixed-width containers were leaving
large idle margins on wide screens — precisely the "wasted space" this
rule now bakes in permanently rather than leaving to be rediscovered.

The pattern that commit established, and that this project keeps:

- **A fixed-width sidebar carries the only hard width constraint on the
  page.** `main` offsets by `margin-left: var(--sidebar-width)` (a CSS
  custom property, collapsed/expanded/mobile-off-canvas) and otherwise
  fills 100% of the remaining viewport. Width comes from the sidebar
  taking a slice off one edge, never from capping the content area
  itself.
- **Grid density keeps increasing at the widest breakpoints — it does not
  plateau.** A poster grid that stops gaining columns past `lg` and just
  grows its side margins instead has reintroduced the exact bug `24cabe5`
  fixed. See the [breakpoint table](#responsive-breakpoints) below for the
  real column counts this project already shipped once.
- **The one legitimate max-width is on prose, and only prose.** A
  paragraph of overview/description text — something meant to be *read*,
  not scanned — gets a readability cap (`max-w-3xl`, ~65–75ch), because
  that's a line-length constraint on text, not a layout constraint on the
  page. `ExpandableText` (this project's shared prose component) is the
  concrete precedent: it caps its own text width and nothing outside it.
  Grids, hero sections, and page chrome are never subject to this cap.
- This isn't a trade-off against the WCAG floor in
  [ux-principles.md](ux-principles.md#keyboard--accessibility-baseline-wcag-22)
  — it's reinforced by it. WCAG 2.2 SC 1.4.10 (Reflow, AA) wants content
  to fill and adapt to available width without introducing unnecessary
  scrolling or dead space; a fluid layout is the more accessible choice,
  not a competing concern.

## Motion

- Aesthetic-Usability Effect: motion should read as *polish*, not
  *decoration* — hover-preview transitions, route transitions, and
  skeleton-loading states are subtle and fast (150–250ms), never a
  showcase animation that delays the user from acting.
- `prefers-reduced-motion` disables non-essential motion entirely (hover
  video/image previews, parallax, auto-playing carousels) per
  [ux-principles.md](ux-principles.md#keyboard--accessibility-baseline-wcag-22)
  — this is a hard floor, not a nice-to-have.
- Loading states below the Doherty Threshold (~400ms) render nothing;
  above it, a skeleton matching the eventual content's shape, not a
  generic spinner — reduces perceived wait per the same principle NN/g
  and Apple's playback guidance both converge on (show the user
  *something specific*, not an abstraction of "loading").

## Iconography

- **One icon set: Lucide (`lucide-react`).** Not a placeholder — settled
  by the pre-reset app's own exclusive use (89 `lucide-react` imports,
  zero imports of any competing icon library across every `.tsx` file).
  Used consistently for the same concept everywhere a status badge,
  action, or nav item appears (NN/g consistency heuristic).
- Status icons (Job succeeded/failed/partial, download states, Item
  lifecycle) pair color *and* shape/icon — never color alone, since
  color-only status coding fails both colorblind users and the WCAG 1.4.1
  "use of color" success criterion.
- **Any icon-only control gets an `aria-label`** — a collapse toggle, a
  mobile-menu button, a close button convey nothing to a screen reader
  without one (WCAG 1.1.1/4.1.2: non-text content needs a text
  alternative, every control needs an accessible name). This project's
  own prior art already did this correctly
  (`aria-label="Open navigation"` on the sidebar's mobile menu button,
  `aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}` on the
  collapse toggle) — restated here as a rule so it's not left to be
  independently rediscovered per component.

### Icon size scale

Raw `size={N}` props in the pre-reset app were messy — `14`×51, `13`×35,
`16`×21, `12`×17, `18`×11, `15`×11, plus one-off odd values — icons were
being hand-tuned per call site, usually to optically match whatever text
sat next to them. Same fix as [Typography](#scale): name a small set of
steps instead, tied to the type-role scale that already exists so an
icon and its adjacent text are always paired by the same underlying step:

| Token | Px | Pairs with | Real precedent |
|---|---|---|---|
| `icon-label` | 12 | `text-label` / `text-label-eyebrow` | `size={11}`–`{13}` cluster — badge/eyebrow icons |
| `icon-body` | 16 | `text-body`, nav items, buttons | `size={14}`–`{18}` cluster — the general-purpose default |
| `icon-title` | 20 | `text-title-md` / `text-title-lg`, larger actions | `size={20}` |
| `icon-display` | 40 | `text-headline` / `text-display`, empty states | `size={32}`–`{48}` cluster |

## Responsive breakpoints

Purser's realistic range is a 4K desktop monitor down to a phone browser
— no dedicated 10-foot/TV remote UI is in scope for the web app (that's a
different, not-yet-planned surface). Breakpoints:

| Breakpoint | Width | Layout change |
|---|---|---|
| `sm` | ≥640px | Single-column detail views become two-column |
| `md` | ≥768px | Sidebar becomes a persistent rail (below `md` it's an off-canvas drawer, per [frontend-stack.md](frontend-stack.md#styling-tailwind-v4)) |
| `lg` | ≥1024px | Library grid goes from 2–3 columns to 4–6 |
| `xl` | ≥1280px | Grid gains another column or two (6→8) — density keeps climbing, it does not plateau |
| `2xl` | ≥1536px | Grid climbs again (e.g. 8→10 for compact cards) — no ceiling is fixed in this doc; add columns for as long as the minimum card width holds, per the [fluid-by-default rule](#fluid-by-default--no-centered-container) |

These aren't invented targets — this project shipped exactly this curve
once (`grid-cols-2 sm:3 md:4 lg:6 xl:8 2xl:10` on `ArtistDetail`'s member
grid, `24cabe5`) and it's restated here as the rule, not left to be
rediscovered per page.

Grid *density* scales with viewport (more columns), not card *size*
shrinking arbitrarily — cards keep a consistent minimum touch/click target
(≥24×24px per WCAG 2.5.8, in practice far larger for a poster card) at
every breakpoint. **The concrete floor behind "no ceiling": columns keep
increasing only as long as each card stays ≥160px wide** — below that,
poster art and title stop being legible at a glance, which is where
density growth should stop even at 4K.

Grid density is what most of this app's responsive behavior actually is
(143 of ~193 total breakpoint-prefixed classes in the pre-reset app were
`grid-cols-*` variants) — but a few other things change too, real and
worth naming rather than leaving implicit:

| Breakpoint | Also changes |
|---|---|
| `< md` | Sidebar is an off-canvas drawer (above); hero/detail-view sections stack `flex-col` instead of side-by-side; `PageHeader`'s search box narrows (`w-40` → `sm:w-52` → `md:w-64`) |
| `lg`+ | Hero sections go from `flex-col` to `lg:flex-row` — image sits beside the title/metadata instead of stacked above it |

## Component vocabulary

Named primitives to reuse, not reinvent per module — cross-referenced to
[ADR 0004](../adr/0004-typescript-react-testing-standards.md)'s existing
shared-component/testing rule (shared components get tested against ≥2
content-type configurations to prove genericity):

- **Card** — poster/cover image, title, status badge; config-driven, not
  forked per content type (music album card and AfterDark scene card are
  the same `Card` with different props, per
  [ux-principles.md](ux-principles.md#state-vocabulary)'s rule that
  content-type knowledge stays out of shared components).
- **Hero** — large treatment for a detail view's top section (image,
  title, primary actions, key metadata) — Netflix/Apple TV's visual
  pattern, purely for polish, per the
  [Reference stack](ux-principles.md#reference-stack) table.
- **StatusBadge** — renders any of the state-vocabulary enums (Job, Item,
  Download, UnmatchedFile) with paired color+icon, no per-content-type
  variant.
- **ActivityRow / JobPanel** — the Plex/Jellyfin dashboard-activity
  precedent named in ux-principles.md; shows Task/Step progress for a
  running Job.
- **EmptyState** — zero-results and empty-library states; always states
  what's missing and offers a next action (NN/g heuristic 9), never a
  bare "Nothing here."
- **EditDialog / entity editors** — carries forward the prior art's
  `components/edit/editors/` naming; the only surface where raw/technical
  fields are exposed, per
  [ux-principles.md](ux-principles.md#progressive-disclosure).

This list is a starting vocabulary, not exhaustive — extend it here as
real screens get built, per [ADR 0004](../adr/0004-typescript-react-testing-standards.md)'s
existing rule against forking near-duplicate components per content type.
