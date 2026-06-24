# TypeScript / React Development Standards

## Architecture

The React UI is an API consumer with no privileged access. It uses the same REST API any other client would use. API handlers in `internal/api/` are the only source of truth — the UI never has access the backend does not expose.

## Data Fetching

Use TanStack Query for all server state. No manual `useEffect` + `fetch` patterns.

## Routing

React Router v6. No custom routing abstractions.

## Styling

Tailwind CSS. No inline styles. No CSS-in-JS.

## Component Rules

**The component-first rule is enforced, not suggested.** Adding a new content-type page or feature without first checking for and extracting a shared abstraction is a defect.

- Shared patterns (cards, detail pages, search dialogs) belong in `web/src/components/` as reusable components
- Content-type-specific behavior is driven by props/config, not forked component trees
- A component built for one content type that would be needed in another must be extracted before the PR lands — not after, not in a follow-up

### Known shared abstractions — use these, do not reimplement

| Pattern | Component/Hook | Location |
|---|---|---|
| Tag browsing page | `TagCloudPage` | `components/TagCloudPage.tsx` |
| Genre list page | `GenreListPage` | `components/GenreListPage.tsx` |
| Genre-filtered entry list | `GenreFilteredPage` | `components/GenreFilteredPage.tsx` |
| Add-entity import dialog | `ImportDialog` | `components/ImportDialog.tsx` |
| Inline edit button | `EditButton` | `components/EditButton.tsx` |
| Album/group card | `AlbumCard` | `components/AlbumCard.tsx` |
| Image cache-busting | `useImageVersion` | `hooks/useImageVersion.ts` |
| Entity edit drawers | `*Editor` | `components/edit/editors/` |

Before adding a new instance of any of the above patterns, verify the shared component exists. If it does not yet exist but the pattern is already duplicated in page files, your first commit must be the extraction.

### Hard bans in components

- `<div onClick>` for interactive controls — use `<button>` (or `<button role="switch">` for toggles)
- Array index as React `key` on any list of fetched entities — use `entity.id`
- Constructing fake/sentinel objects to satisfy a prop type — fix the prop type to accept the narrower available type
- `useState` counter for image cache-busting — use `useImageVersion`
- Dual paginated queries merged client-side for pagination — use a server-side combined query
- `O(N)` computation in a component render loop without `useMemo`

## Code Conventions

- No comments explaining what code does — names do that
- Follow React, Vite, TypeScript, and CSS best practices
- No `any` types without justification

## Testing

The frontend must include unit tests alongside components. No shipping frontend code without test coverage.
