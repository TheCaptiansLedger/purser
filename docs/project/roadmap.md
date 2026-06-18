# Purser Roadmap

Features are organized in dependency order. Each task is scoped to fit in roughly one hour and produces a clean, independently testable increment. Tasks within a feature are sequenced — later tasks depend on earlier ones within the same feature.

---

## Fix: Config Test Environment Isolation

`TestLoad_Defaults_SourcesDisabled` and `TestLoad_SourcesFromYAML` in
`internal/config/config_test.go` fail on any machine that has real Purser
environment variables set (e.g. `PURSER_STASHDB_API_KEY`). The tests assert
default/fixture values but pick up the host environment instead.

### Tasks

**1. Isolate config tests from the host environment**
- In `config_test.go`, call `t.Setenv` to explicitly set every env var the
  config loader reads, and unset any that should be absent for a given test case
- Where a test loads from a YAML fixture, additionally clear all
  `PURSER_*` env vars at the start of that test using `t.Setenv` with empty
  values or a helper that snapshots and restores `os.Environ`
- Tests must pass on any machine regardless of what is set in the shell
- Confirm with `go test ./internal/config/...` in an environment where real
  keys are present

---

## First-Time Setup Wizard

**Status:** Planned — not yet implemented

When Purser starts against a fresh database it currently silently initialises the
schema and drops the user straight into an empty UI with no guidance.

The planned wizard intercepts the first request to the UI (detected by a
`setup_complete` flag in a `settings` table) and walks the user through:

1. **Welcome** — brief product description and what Purser does
2. **Enable modules** — toggle Movies / TV / Music / Books / AfterDark / JAV
3. **Connect metadata sources** — enter API keys for StashDB, TMDB, TVDB, etc.
   with a "Test connection" button for each
4. **Media roots** — configure the scan roots for each enabled module
5. **Done** — redirect to the library

Until the wizard exists, first-time setup is manual: edit `ops/purser.yaml`,
set environment variables in `.env`, then run `make dev`.

---

## Tag Category ✓

Adds a `category` field to tags so genres can be distinguished from content
warnings and general metadata labels. This is a prerequisite for the genre nav
and for structured tag display on person/item detail pages.

**Possible category values:** `""` (default/general), `"genre"`, `"content_warning"`.
Metadata sources set category at import time since they know which of their own
tags are genres vs. descriptive labels.

### Tasks

- [x] **1. Domain + port — `TagCategory` type and `Tag.Category` field**
- [x] **2. DB — migration `003_tag_category.sql` + tag repo**
- [x] **3. UI — TypeScript `Tag` type + API client**

---

## Monitor Mode: `latest` ✓

Adds `MonitorLatest` as the fourth monitor mode and makes it the default when
adding a studio. Under `latest`, only the single most recently released item is
`monitored=true` at initial import; on subsequent refreshes new scenes arrive as
`monitored=false` and the user picks what they want.

### Tasks

- [x] **1. Domain — `MonitorLatest` constant**
- [x] **2. App service — default monitor mode for `ImportStudio`**
- [x] **3. UI — add `latest` to monitor mode selector**

---

## Async Job System ✓

Cross-cutting infrastructure for all background operations. Every long-running
task (studio refresh, library scan, indexer search) submits a `Job` and reports
progress through a standard interface. The UI polls for status and can cancel
in-flight jobs.

This feature has no dependencies and should be built before studio auto-populate.

### Tasks

- [x] **1. Domain — `Job` type and `JobStatus` constants**
- [x] **2. Port — `JobQueue` interface and `ProgressReporter`**
- [x] **3. Adapter — in-memory `JobQueue` implementation**
- [x] **4. API — job endpoints**
- [x] **5. UI — `JobsPanel` component**
- [x] **6. UI — sidebar job indicator**

---

## Studio Auto-Populate ✓

When a studio is added, Purser fetches metadata for every known scene from the
configured sources and creates `Item` records. Items arrive as
`status=wanted, monitored=<per monitor_mode>`. The import runs as a background
`Job` so the UI is never blocked.

Depends on: **Async Job System**, **Monitor Mode: `latest`**

### Tasks

- [x] **1. Port — `FetchEntryContent` on `MetadataSource`**
- [x] **2. StashDB adapter — implement `FetchEntryContent`**
- [x] **3. App service — `RefreshStudio` method**
- [x] **4. App service — wire `AutoImport` into `ImportStudio`**
- [x] **5. API — `RefreshStudio` command + studio add wiring**
- [x] **6. RefreshStudio — scene cover art download**
- [x] **7. RefreshStudio — performer import and scene linking**
- [x] **8. RefreshStudio — tag import and deduplication**
- [x] **9. RefreshStudio — incremental UI feedback (scenes pop in as they arrive)**

---

## Item Status Overlay

Visual system that makes acquisition state immediately readable on every item
card. Hover-only by default; a toggle in the view header switches to
always-visible mode, persisted per-module in `localStorage`.

### Tasks

**1. `ItemStatusBadge` component**
- Create `web/src/components/media/ItemStatusBadge.tsx`
- Accepts `status: ItemStatus` and `monitored: boolean`
- Renders a colored icon + short label:
  - `imported` → green check + quality string if available (`"1080p"`)
  - `wanted` + `monitored=true` → blue bookmark
  - `grabbed | downloading` → orange download/spinner icon
  - `missing` → red warning triangle
  - `monitored=false` (any status) → muted gray X
- No click handling in this component — display only
- Tests: render each state, assert correct class/icon is present

**2. Overlay on `ItemCard`**
- In `web/src/components/media/ItemCard.tsx`, position `ItemStatusBadge` in the
  bottom-right corner of the card image
- Overlay is hidden by default (`opacity-0`) and transitions to visible on
  `group-hover` (Tailwind `group` on the card wrapper)
- When `alwaysShowStatus` prop is `true`, override to `opacity-100`
- Cards that have no status data (entries, not items) are unaffected

**3. View-level status visibility toggle**
- Add a toggle button to the scene/item grid view header (next to sort controls)
  using an eye icon
- Toggle state stored in `localStorage` keyed by `purser:statusOverlay:<module>`
  (e.g. `purser:statusOverlay:afterdark`)
- Expose value via a `useStatusOverlay()` hook; pass down as `alwaysShowStatus`
  prop to `ItemCard` through the grid component

**4. `ItemStatusPopover` — click to change state**
- Create `web/src/components/media/ItemStatusPopover.tsx`
- Wraps `ItemStatusBadge`; clicking opens a small popover with context-appropriate
  actions:
  - `monitored=false` → **[Mark as Wanted]** / **[Skip]**
  - `wanted` + `monitored=true` → **[Skip]** / **[Mark as Manually Imported]**
  - `imported` → show file path + quality; no grab actions
  - `grabbed | downloading` → show status text only; no actions
- Each action calls `PATCH /api/v1/items/:id` and invalidates the TanStack Query
  cache for the parent list

**5. API — `PATCH /api/v1/items/:id`**
- Add handler to `internal/api/` accepting `{ "monitored": bool }` and/or
  `{ "status": "skipped" | "wanted" }`
- Validates that status transitions are legal (e.g., cannot manually set
  `downloaded`)
- Tests: `httptest` — patch monitored, assert item returned with new value

---

## Artist Relationships (Music Only)

Allows bands to have explicit member relationships to `Person` records, mirroring
how performers are shown on scenes. Scoped to `kind=artist` library entries only.

### Tasks

**1. DB — `entry_people` table + domain type**
- Write `internal/adapters/db/migrations/004_entry_people.sql`:
  ```sql
  CREATE TABLE entry_people (
      library_entry_id  TEXT NOT NULL REFERENCES library_entries(id) ON DELETE CASCADE,
      person_id         TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
      role              TEXT NOT NULL DEFAULT 'member',
      start_date        TEXT,
      end_date          TEXT,
      PRIMARY KEY (library_entry_id, person_id, role)
  );
  CREATE INDEX entry_people_entry_idx ON entry_people(library_entry_id);
  CREATE INDEX entry_people_person_idx ON entry_people(person_id);
  ```
- Add `type EntryPerson struct` to `internal/domain/library_entry.go`:
  `PersonID, Role, StartDate, EndDate string; Person *Person`
- Add `People []EntryPerson` field to `domain.LibraryEntry` (nil unless loaded)

**2. DB adapter — `GetPeople`, `SavePerson`, `RemovePerson`**
- Add three methods to `internal/adapters/db/library_entry.go`:
  - `GetPeople(ctx, entryID string) ([]domain.EntryPerson, error)` — joins
    `entry_people` + `people` + `people_aliases`
  - `SavePerson(ctx, entryID string, ep domain.EntryPerson) error` — upsert
  - `RemovePerson(ctx, entryID, personID, role string) error` — delete row
- Add these to the `LibraryEntryRepository` port in `internal/ports/repository.go`
- Tests: `repo_test.go` — save entry, save person, link via SavePerson, GetPeople
  returns one result

**3. App service — `ImportArtistMembers`**
- Add `ImportArtistMembers(ctx context.Context, entryID string, members []*domain.ExternalPerson, role string) error`
  to `app/metadata/service.go`
- For each member: upsert via existing `ImportPerson` logic, then call
  `entries.SavePerson`
- The MusicBrainz adapter (future) will call this via the standard
  `FetchStudioContent` equivalent; for now expose the method so it's wired
- Tests: mock repo; assert SavePerson called once per member

**4. API — include `people` in artist entry response**
- In `internal/api/library.go` (or equivalent handler), when serving
  `GET /api/v1/library-entries/:id` and `entry.Kind == "artist"`, call
  `repo.GetPeople` and include a `"people"` array in the JSON response
- Add `people?: EntryPerson[]` to the `LibraryEntry` TypeScript interface
- Tests: `httptest` — GET artist entry, assert `people` array present in body

**5. UI — Members section on artist detail page**
- In the music artist detail page, add a "Members" section below the discography
  using the existing `PersonCard` grid component (same layout as performers on
  a scene/studio page)
- Only rendered when `entry.people` is non-empty
- Each card links to `/music/artists/:personId` (person detail page)
- Person detail page: add "Member of" section listing band entries that include
  this person via `entry_people`

---

## Person Metadata Keys

Documents the normalized key set for `Person.Metadata` across all sources,
extends the StashDB adapter to fetch currently-missing fields, and renders
known keys on the person detail page.

### Tasks

**1. Write `docs/person-metadata-keys.md`**
- Document the full normalized key set (see file for detail)
- Organized by: Universal keys, Performer keys (StashDB/adult), Actor keys (TMDB),
  Artist keys (MusicBrainz)
- Include display labels, value format, and which sources provide each key
- This file is the authoritative reference for UI rendering and adapter mapping

**2. StashDB adapter — fetch missing performer fields**
- Extend `gqlPerformer` struct in `internal/adapters/stashdb/people.go` with:
  `Gender, Ethnicity, Nationality, Disambiguation string`
  `Weight int` (kg)
  `FakeTits *bool`
  `CareerEndYear int`
- Update `searchPeopleQuery` and `findScenesByFingerprintQuery` performer fragment
  to request these fields
- Update `performerMetadata()` to populate: `gender`, `ethnicity`, `nationality`,
  `weight` (`"X kg"`), `career_end`, `fake_tits`, `disambiguation`
- Normalize gender from StashDB enum (`MALE`→`"male"`, `FEMALE`→`"female"`,
  `TRANSGENDER_MALE`→`"transgender_male"`, etc.)
- Tests: fixture JSON with all fields set; assert metadata map has correct keys

**3. UI — person detail page renders metadata**
- Add a structured metadata grid to the person detail page (afterdark performer
  page; same component will be reused for actors and musicians)
- Render known keys in logical groups:
  - **Physical:** height, weight, measurements, cup_size, hair_color, eye_color
  - **Background:** birthdate, nationality (with flag), ethnicity, gender
  - **Career:** career_start, career_end, fake_tits
  - **Bio:** biography / overview (TMDB), disambiguation (MusicBrainz)
- Unknown keys in `metadata` are silently skipped — no catch-all rendering
- `nationality`: render Unicode flag emoji + country name (see Country Flag Utility)

---

## Country Flag Utility

Small shared utility for displaying nationality with a flag. Used by the person
metadata renderer and any other place a country is displayed.

### Tasks

**1. `countryFlag` utility + `CountryChip` component**
- Create `web/src/utils/countryFlag.ts`:
  - `countryFlag(isoCode: string): string` — converts ISO 3166-1 alpha-2 to
    Unicode flag emoji using regional indicator symbols
    (`"US"` → `"🇺🇸"`, `"GB"` → `"🇬🇧"`)
  - `countryName(isoCode: string): string` — resolves code to English name via
    a small bundled lookup (top ~50 countries by content prevalence; falls back
    to the raw code)
- Create `web/src/components/ui/CountryChip.tsx`:
  - Accepts `code: string` (ISO alpha-2 or plain string)
  - If code is recognized: renders `<flag emoji> <country name>` in a subtle chip
  - If not recognized: renders the raw string as plain text
- `ethnicity` is rendered as plain text — no flag mapping (it's descriptive, not
  a country code)

---

## Jobs UX — Sidebar Spinner + Detail Panel

The jobs panel currently only lives on the Settings page. Background jobs have
no presence in the rest of the UI, and there is no way to inspect job details
without navigating away from what you're doing.

### Tasks

**1. Sidebar job activity indicator**
- When any job is `queued` or `running`, show a small animated spinner (or pulsing
  dot) next to the Jobs nav item in the sidebar
- The indicator disappears when all jobs reach a terminal state
- `useJobs` is already polling at 2 s intervals; read from that query in the
  sidebar component and derive `hasActiveJobs` from the returned list
- No new API calls — the existing poll covers it

**2. Job detail panel**
- Clicking a job row in the jobs panel opens a detail view — either a right-hand
  column that slides in or a popover/sheet anchored to the row
- Detail view shows: job name, status, payload fields, progress bar (current/total),
  log message, error (if failed), created/started/completed timestamps
- The panel closes when the user clicks elsewhere or presses Escape
- The list and panel coexist on screen; selecting a different job row updates the
  panel in place without closing it

**3. Jobs panel — adaptive layout**
- When the jobs panel is the primary content on screen (Settings → Jobs tab),
  the job list should expand to fill the available width rather than being
  constrained to a narrow column
- If the detail panel is open, the list and panel share the space with a
  reasonable split (e.g. 40/60); if no job is selected, the list takes full width
- The current fixed-width card layout reads as wasted whitespace at typical
  desktop resolutions

---

## Scene UX — Metadata, Status, Navigation, and Filtering

Scenes are currently listed with title, year, and performer names only. Several
pieces of information are missing from both the card and the detail page, and
navigation between scenes and their parent studio is broken.

### Tasks

**1. Scene card — status and monitored indicator**
- Add a visual indicator to `ItemCard` for monitored/status state:
  - Monitored + wanted: subtle border or badge (e.g. accent-colored ring)
  - Skipped / not monitored: dimmed card or strikethrough on title
  - Imported: existing "HD" badge is sufficient; keep it
- The indicator should be unobtrusive — visible on hover by default, always
  visible when the global status overlay toggle is on (see Item Status Overlay
  feature)

**2. Scene detail — back navigation**
- The scene detail page (`SceneDetail.tsx`) has no link back to the scene list
  or to the parent studio
- Add a breadcrumb row at the top: `Studios › <Studio Name> › <Scene Title>`
  where "Studios" links to `/afterdark/studios` and `<Studio Name>` links to
  `/afterdark/studios/:studioId`
- The studio ID and name must be resolved from the scene's `libraryEntryId`;
  call `GET /api/v1/library-entries/:id` with that value

**3. Scenes list — sort by date + pagination**
- The scenes grid on both `ScenesPage` (all scenes) and the studio detail scenes
  section currently loads up to 200 items with no sorting control and no pagination
- Add a sort control (`Date ↓ / Date ↑ / Title A–Z`) rendered in the page header
  or above the grid; default to `Date ↓` (newest first)
- Wire sort selection to the `useItems` query filter (add `sort` and `sortDir`
  params to `ItemFilter` and pass them through to `GET /api/v1/items`)
- Add backend support: `GET /api/v1/items?sort=date&sortDir=desc` — extend the
  `ItemFilter` in `internal/ports/repository.go` and the corresponding SQL query
- Add pagination controls below the grid using the existing `Pagination` component;
  default page size 48
- The studio detail page should switch from fetching 200 items at once to a
  paginated query with the same sort control
