# 0004. TypeScript/React Testing Standards

Status: Accepted

## Context

The `web/` frontend repeatedly accumulated near-duplicate pages and
components per content type (tag clouds, genre pages, detail editors,
import dialogs) because nothing forced a check for an existing shared
component before writing a new one, and no test made a content-type branch
inside a shared component visible as a defect. This ADR sets the testing bar
that catches both.

## Decision

Test with Vitest + React Testing Library. Coverage is tracked per the
frontend component in `codecov.yml` if/when a `web` component is added there
(add one if missing rather than leaving `web/` uncovered).

- **Shared components** (`web/src/components/**`): every shared component
  (`TagCloudPage`, `GenreListPage`/`GenreFilteredPage`, `ImportDialog`,
  `EditButton`, `AlbumCard`, entity editors under `components/edit/editors/`,
  field components like `Toggle`/`RuntimeInput`, hooks like
  `useImageVersion`) must have a test that exercises it with at least two
  different content types/props configurations. A test that only exercises
  one content type through a "shared" component hides a content-type branch
  instead of catching it.
- **Pages** (`web/src/pages/**`): test that the page composes the shared
  components correctly (renders them, passes the right props) — do not
  re-test the shared component's internal behavior at the page level.
- **Hooks** (`web/src/hooks/**` or colocated): unit tested in isolation via
  `renderHook`, independent of any specific page.
- **API layer** (`web/src/api/**` or equivalent fetch hooks): tested against
  a mocked HTTP layer (e.g. MSW or fetch mock) — never against a live
  backend. Assert the request shape and response parsing, not business
  logic.
- Every field returned by a Go API response that a component reads must
  exist on the corresponding interface in `web/src/types/index.ts`. This is
  a compile-time check only if some component actually reads the field —
  when adding a field, add or update a test that reads it, so the type gap
  can't hide silently.

Do not write a component test that mocks away the thing being tested (e.g.
shallow-rendering a component and asserting only that a mock function was
called, with no assertion on rendered output) — that produces coverage
percentage with no defect-catching power.

## Consequences

- Writing a "shared" component now requires proving it's actually shared
  (multi-content-type test) before it counts as done — this is the
  mechanical enforcement of the Component-First requirement.
- Slightly more test setup per shared component (multiple prop
  configurations); pages stay cheap to test since they don't re-test
  children.

## Self-Audit Checklist

1. Does every shared component's test exercise it with more than one
   content type or prop configuration? If it only covers one — the test
   isn't proving the component is actually generic.
2. Did I add a new page-level pattern (tag page, genre page, editor, import
   dialog) without first checking `web/src/components/` and `web/src/pages/`
   for an existing one? If yes — stop and extract/reuse instead.
3. Does any component test assert against a content-type or module-name
   branch inside the component under test? If yes — that's a defect in the
   component, not something to snapshot past.
4. Did I add a field to a Go response struct without checking
   `web/src/types/index.ts` has the matching field, and without a test that
   reads it?
