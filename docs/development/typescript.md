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

- Shared patterns (cards, detail pages, search dialogs) belong in `web/src/components/` as reusable components
- Content-type-specific behavior is driven by props/config, not forked component trees
- A component built for one content type that would be needed in another must be extracted before the PR lands

## Code Conventions

- No comments explaining what code does — names do that
- Follow React, Vite, TypeScript, and CSS best practices
- No `any` types without justification

## Testing

The frontend must include unit tests alongside components. No shipping frontend code without test coverage.
