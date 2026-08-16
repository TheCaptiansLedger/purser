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
   components: `--palette-ink-950: #1d2021`, `--palette-aqua-500: ...`,
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
neutral tokens they're paired with.

**Palette: Gruvbox Dark**, adapted rather than copied verbatim — the
prior neutral-indigo palette is replaced token-for-token, every pairing
re-verified against this app's own actual usages (body text, status
badges, a button/toggle's solid-fill-with-text-on-top pattern), not
assumed compatible just because Gruvbox itself is a known-good scheme.
Since every component already reads colors through the semantic tokens
below rather than a hardcoded hex, this is a one-file swap
(`web/src/index.css`) with app-wide effect, not a component-by-component
rewrite — exactly the point of the token architecture above.

### Neutral scale

| Token | Value | Use |
|---|---|---|
| `--color-bg` | `#1d2021` | App background (Gruvbox `bg0_h`) |
| `--color-surface` | `#282828` | Cards, panels (Gruvbox `bg0`) |
| `--color-surface-raised` | `#3c3836` | Modals, popovers, hover state (Gruvbox `bg1`) |
| `--color-border` | `rgba(235,219,178,0.08)` | Default dividers (Gruvbox `fg1` tint) |
| `--color-border-hover` | `rgba(235,219,178,0.18)` | Interactive-element border on hover/focus |
| `--color-text` | `#ebdbb2` | Primary text (Gruvbox `fg1`) |
| `--color-text-secondary` | `#bdae93` | Secondary text, metadata (Gruvbox `fg3`) |
| `--color-text-muted` | `#665c54` | Disabled/placeholder only — see restriction below (Gruvbox `bg3`) |

Computed body-text contrast (WCAG 1.4.3, AA needs ≥4.5:1):

| Pair | Ratio | Passes AA? |
|---|---|---|
| `--color-text` on `--color-bg` | 11.95:1 | Yes |
| `--color-text` on `--color-surface` | 10.75:1 | Yes |
| `--color-text` on `--color-surface-raised` | 8.45:1 | Yes |
| `--color-text-secondary` on `--color-bg` | 7.53:1 | Yes |
| `--color-text-secondary` on `--color-surface` | 6.77:1 | Yes |
| `--color-text-secondary` on `--color-surface-raised` | 5.32:1 | Yes |
| `--color-text-muted` on any surface | 1.78–2.52:1 | **No** |

**`--color-text-muted` fails AA outright and must stay restricted to
disabled/placeholder UI, never used for text that conveys real
information.** WCAG's contrast criteria don't apply to inactive controls,
which is the only reason this token is usable at all — treat that as a
hard boundary on where it appears, not an oversight to eventually fix.
(Concretely: a settings field's unit hint — "e.g. 45s" — is real
information and must render at `--color-text-secondary`, not
`--color-text-muted`, even though a hint reads as secondary/minor.)

### Content-type accents

One hue per `content_type` (open vocabulary — see
[ux-principles.md](ux-principles.md#state-vocabulary)) — used for nav
icons, hero-backdrop tint, and card borders. **Never used as body text**;
verified only against the 3:1 non-text-contrast floor (WCAG 1.4.11), not
the 4.5:1 text floor.

| Content type | Token | Hex | vs. surface | vs. bg |
|---|---|---|---|---|
| `music` | `--color-accent-music` | `#b8bb26` | 7.14:1 | 7.94:1 |
| `afterdark` | `--color-accent-afterdark` | `#fb4934` | 4.29:1 | 4.77:1 |

Gruvbox's bright green and bright red — chosen to land on the same hue
family (green/red) the prior indigo-era palette used for these two
content types, so the association carries over even though the exact
hexes don't. Adding a third content type later: pick the next unused
Gruvbox "bright" hue (`yellow` `#fabd2f`, `purple` `#d3869b`, and `blue`
`#83a598`/`aqua` `#8ec07c` are already spoken for by the system accent and
status buckets below — prefer `orange` `#fe8019` next), verify it here
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
| System/neutral (Library umbrella, Acquisition, Pipeline/Jobs, People, Tags, Settings) | `--color-accent-system` | `#83a598` | 5.48:1 |

**`--color-accent-system` as a solid button/toggle fill needs dark text on
top, not light.** The token itself (Gruvbox bright blue) is light enough
that `--color-text` on top of it is only 1.96:1 — badly fails AA. Where a
component paints text directly on a solid `bg-accent-system` fill (the
Config tab's Save button; a checked `Toggle`'s thumb; nothing else does
today), pair it with `--color-bg` instead: `6.09:1`, comfortably passes.
Every other `accent-system` use (nav icons, borders, tints) is a non-text
UI element and stays at the 3:1 floor verified above.

### Status/semantic tokens

A single shared 7-bucket palette that **every** state-bearing entity
maps onto, instead of four independent color systems. Rendered the same
way the pre-reset `ItemStatusBadge` shipped it: badge text is the full
color, badge background is that same color at **8% alpha** over
`--color-surface` (down from the pre-Gruvbox 13% — Gruvbox's `bg0` is
lighter than the prior near-black surface, so the same 13% tint left less
room between tint and text; 8% restores the same comfortable margin).

| Bucket | Token | Hex | Text-on-own-tint contrast |
|---|---|---|---|
| Pending / not started | `--color-status-pending` | `#83a598` | 4.81:1 |
| Queued / next-up | `--color-status-queued` | `#d3869b` | 4.71:1 |
| Active / in progress | `--color-status-active` | `#fabd2f` | 7.30:1 |
| Success | `--color-status-success` (= `--color-success`) | `#b8bb26` | 6.10:1 |
| Warning / partial / paused | `--color-status-warning` (= `--color-warning`) | `#fe8019` | 5.17:1 |
| Failure | `--color-status-failure` (= `--color-danger`) | `#fb6b51` | 4.62:1 |
| Neutral / skipped / dismissed | `--color-status-neutral` | `#a89984` | 4.67:1 |

Every bucket clears 4.5:1 as badge text. `--color-status-failure` is a
**lightened** variant of Gruvbox's canonical bright red (`#fb4934`,
mixed ~20% toward Gruvbox's `fg0` white) rather than that hex directly —
saturated red's low WCAG luminance weight means the canonical value
plateaus at 3.7–3.9:1 against any tint dark enough to still read as "a
red badge," no matter how the tint alpha is tuned; every other bucket
reaches 4.5:1 at 8% alpha without adjustment. `--color-accent-afterdark`
(the content-type accent, a purely non-text 3:1 use) keeps the canonical,
unlightened red — the two tokens are independent even though they're
both "red."

`--color-danger`/`--color-success`/`--color-warning` (referenced earlier
in this doc under Iconography/Motion) are aliases of the `failure`/
`success`/`warning` buckets — a form-validation error and a `Job`'s
"Failed" badge are deliberately the same visual signal, not two
unrelated reds.

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
| `MusicRelease` | Stub | — | — | Imported | Partial | — | — |

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
- **`*StatusBadge`** — one small badge component per entity's own closed
  status enum (`JobStatusBadge`, `ItemStatusBadge`,
  `MusicReleaseStatusBadge`, ...), not a single generic cross-entity
  `StatusBadge`. Each maps its enum's values to a label + status-token
  color via the "Entity → bucket mapping" table above; color is never the
  only signal, always paired with distinct label text.
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
- **ImageLightbox** — clicking any rendered image expands it full-screen,
  reusing the same served bytes the thumbnail already loaded (pre-reset
  precedent: issue #288). Content-type agnostic — a Person's photo, a
  Music release's cover, an AfterDark scene image all open the same way.
  See [docs/technical/image-caching-and-serving.md](../technical/image-caching-and-serving.md).

This list is a starting vocabulary, not exhaustive — extend it here as
real screens get built, per [ADR 0004](../adr/0004-typescript-react-testing-standards.md)'s
existing rule against forking near-duplicate components per content type.
