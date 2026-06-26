# !! HARD BAN !!

NEVER run `git commit`, `git push`, or `gh pr create`. Not ever. Not for any reason.
The user runs git themselves. Claude produces the commit message as text and stops.

---

# Purser — Agent Index

Do not read all project documentation. Determine what the task touches, then load only the required files listed below.

## Always Load

- [docs/project/vision.md](docs/project/vision.md)

## Never Load

- docs/user/

---

## Load by Task

### When making any change that will be committed to the repository
- [docs/development/process.md](docs/development/process.md)

### Starting a new feature or issue
- [docs/development/process.md](docs/development/process.md)
- [docs/version-control/workflow.md](docs/version-control/workflow.md)

### Writing Go code
- [docs/development/go.md](docs/development/go.md)
- [docs/development/testing.md](docs/development/testing.md)
- [docs/architecture/overview.md](docs/architecture/overview.md)
- [docs/architecture/principles.md](docs/architecture/principles.md)

### Writing TypeScript or React
- [docs/development/typescript.md](docs/development/typescript.md)
- [docs/development/testing.md](docs/development/testing.md)
- [docs/architecture/principles.md](docs/architecture/principles.md)

### Designing a new feature (any content type)
- [docs/architecture/feature-design.md](docs/architecture/feature-design.md)
- [docs/architecture/principles.md](docs/architecture/principles.md)

### Changing domain model or database schemas
- [docs/technical/data-model.md](docs/technical/data-model.md)
- [docs/architecture/overview.md](docs/architecture/overview.md)

### Changing ports or adapters
- [docs/technical/ports.md](docs/technical/ports.md)
- [docs/architecture/overview.md](docs/architecture/overview.md)
- [docs/architecture/principles.md](docs/architecture/principles.md)

### Changing API behavior or routes
- [docs/technical/api.md](docs/technical/api.md)

### Committing or creating a PR
- [docs/version-control/workflow.md](docs/version-control/workflow.md)

### Working with external metadata sources
- [docs/technical/external-sources.md](docs/technical/external-sources.md)

### Working with person/people data or metadata
- [docs/technical/person-metadata-keys.md](docs/technical/person-metadata-keys.md)

### Working on the acquisition pipeline (Phase 2)
- [docs/technical/acquisition-pipeline.md](docs/technical/acquisition-pipeline.md)
- [docs/technical/ports.md](docs/technical/ports.md)
- [docs/architecture/overview.md](docs/architecture/overview.md)

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
Every field the Go API returns must exist on the corresponding TypeScript interface in `web/src/types/index.ts`. When reviewing or writing a change that adds a field to a Go response struct, verify the matching TypeScript interface has that field. When a field is missing from the TypeScript type, the compiler gives no error unless a component actually tries to use it — so the gap is invisible until runtime.

### Component-First gate (run before writing any TypeScript or React code)

Before writing or proposing any new page, component, or hook, answer these questions. This is a required check, not a style suggestion.

1. **Does an equivalent pattern already exist for another content type?**
   Grep `web/src/pages/` for the pattern (tag cloud pages, genre pages, detail editors, add-entity dialogs). If found: extract first, then use. Never add a third copy.

2. **Is this abstraction already in `web/src/components/`?**
   Known shared components that must be used instead of re-implementing:
   - `TagCloudPage` — tag browsing pages (any content type)
   - `GenreListPage` / `GenreFilteredPage` — genre browsing pages
   - `ImportDialog` — add-entity multi-step search/import dialog
   - `EditButton` — the inline edit button on detail pages
   - `AlbumCard` — album/group card in music contexts
   - `useImageVersion` — cache-busting for editable entity images
   - `components/edit/editors/` — all entity edit drawers
   - `Toggle` — boolean on/off switch (`components/edit/fields/Toggle.tsx`)
   - `RuntimeInput` — h/m/s runtime compound field (`components/edit/fields/RuntimeInput.tsx`)
   If the component doesn't exist yet but the pattern already appears elsewhere in page files, extract it first.

3. **Does the proposed component branch on content type, kind, or module name?**
   If yes: do not write it. Fix the design — behavior must come from props/config, never from inline string comparisons.

4. **Am I about to duplicate state management logic?**
   - Image version counter → `useImageVersion`
   - localStorage ↔ React state sync → lift state, pass as props
   - Dual paginated queries merged client-side → single server-side combined query

Writing an inline page-level implementation when a shared component should exist is a defect. The audit will find it and the refactor will happen anyway — do it before, not after.
