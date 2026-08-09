# Web UI — Frontend Stack & Configuration

Status: Draft — living document, not ADR-numbered. This is the
"configuration" half of the plan: what tooling implements
[ux-principles.md](ux-principles.md) and [style-guide.md](style-guide.md).
**The framework scaffold described here is implemented** — Tailwind,
routing, the Connect-Web/Connect-Query data layer, the fixed-sidebar
layout shell, and Go-side embedding/serving of the build are all wired
end-to-end behind a single Welcome page. No module (Music/AfterDark/
Acquisition/Pipeline) screens exist yet — that's the next, separate pass.

## Data layer: Connect-Web, not REST

The binding constraint, from [ADR 0011](../adr/0011-api-design.md):

> The web UI talks RPC (Connect protocol / gRPC-Web via a
> connect-web-generated client), not HTTP/JSON — HTTP/JSON is for other
> consumers (curl, k6, tooling), not the primary UI path.

Concretely:

- **`@connectrpc/connect-web`** generates a typed client from the same
  `.proto` files that already produce `gen/go/...` — one contract, no
  hand-written fetch calls, no REST assumptions anywhere in the frontend.
- **`buf.gen.yaml` targets Go and TypeScript** — `protoc-gen-go`/
  `protoc-gen-connect-go` → `gen/go` (unchanged), plus `protoc-gen-es`/
  `protoc-gen-connect-query` → **`web/src/gen/`** (settled — living
  alongside the code that consumes it, matching this doc's own directory
  layout below, rather than a top-level `gen/ts/` mirroring `gen/go/`).
  Generated code is committed, same convention as `gen/go/`. The TS
  plugins are npm devDependencies of `web/`, reachable on `PATH` via
  `make proto-gen`'s prerequisite on `web/node_modules/.bin` — see the
  Makefile.
- **`@connectrpc/connect-query`** (settled, not raw TanStack Query +
  hand-written wrappers) — it generates a `useQuery(method, request)`
  hook directly from the same buf codegen pass, which is less code than
  hand-rolling query-key wrappers, not more. It wraps TanStack Query
  underneath, so the loading-state rules in
  [style-guide.md](style-guide.md#motion) (nothing under the Doherty
  Threshold, a shaped skeleton above it) still apply without every
  component hand-rolling its own loading/error state.
- **Composed views** (e.g. a full performer profile) have no backing
  composing-service RPC yet, per ADR 0011's own admitted gap. Until one
  exists, a detail page that needs it issues multiple Connect calls
  client-side, coordinated through TanStack Query's parallel-query
  support — not a blocking problem, but explicitly not hidden behind a
  single RPC that doesn't exist.

## Routing: React Router

React Router v7 (prior-art's v6 revisited, not assumed — v7 is the
current maintained major and, in the plain `createBrowserRouter`/
`RouterProvider`/`Outlet` shape this project uses, its API is unchanged
from v6.4+'s data router) — route-based
navigation for anything bookmarkable, per
[ux-principles.md](ux-principles.md#navigation--information-architecture)'s
"modal drill-down for editing, route-based for browsing" rule. Route
structure mirrors the primary nav's modules (Library, Music, AfterDark,
Acquisition, Pipeline/Jobs), not the proto package structure — a user
never needs to know `purser.pipeline.v1` exists.

## Styling: Tailwind v4

- **Tailwind v4, `@theme` custom-property syntax** — carried forward from
  the deleted `docs/development/typescript.md`'s hard rule, re-affirmed
  rather than silently dropped: **no inline styles, no CSS-in-JS.**
- The semantic token layer in
  [style-guide.md](style-guide.md#theming-architecture) maps directly to
  Tailwind's `@theme` block — `--color-bg`, `--color-surface`, etc. become
  both CSS custom properties and Tailwind utility classes
  (`bg-surface`, `text-text-secondary`) from the same source of truth.
- **Icon library: Lucide (`lucide-react`)** — settled, not a placeholder;
  see [style-guide.md](style-guide.md#iconography) for the icon-size
  scale and the "one set" rule this implements.
- **Page shells do not use Tailwind's `container` utility** — it centers
  and caps max-width by default, which is exactly the pattern
  [style-guide.md's fluid-by-default rule](style-guide.md#fluid-by-default--no-centered-container)
  forbids. Layout width instead comes from a fixed-width sidebar and a
  fluid content area, the same shape this project's own prior art
  (`Layout.tsx`/`Sidebar.tsx`, pre-reset) already used: a `--sidebar-width`
  CSS custom property (set per collapsed/expanded state, `0px` below the
  `md` breakpoint), `main` offset by `margin-left: var(--sidebar-width)`
  and otherwise unconstrained, and the sidebar itself becoming an
  off-canvas drawer (translate-based, backdrop-dismissed) below `md`
  rather than disappearing.

## Directory layout

Matches what [ADR 0004](../adr/0004-typescript-react-testing-standards.md)
already assumes exists, none of which is built yet:

```
web/src/
├── components/       # shared, config-driven primitives (Card, Hero,
│                      #   StatusBadge, EmptyState, ActivityRow, ...)
│   └── edit/editors/  # entity-specific edit forms — the one place raw
│                       #   fields surface, per ux-principles.md
├── pages/             # route-level composition only — assembles
│                       #   components, does not re-implement them
├── hooks/             # data-fetching + UI-state hooks (TanStack Query
│                       #   wrappers per entity/RPC)
├── api/               # generated Connect client + any thin wrappers
├── types/index.ts      # TypeScript types mirroring API responses — every
│                       #   field a component reads must exist here, with
│                       #   a test proving it (ADR 0004)
└── gen/                # committed Connect-ES generated code
```

## What's built vs. what's next

Built: Tailwind + design tokens, React Router, Connect-Web/Connect-Query,
the `Layout`/`Sidebar` shell, `buf.gen.yaml`'s TS target, and Go-side
embedding/serving of `web/dist` (`web/embed.go`,
`cmd/purser/webassets.go`) — all proven end-to-end by the Welcome page's
real `JobService.ListJobs` call.

Not built: every module screen (Music, AfterDark, Acquisition,
Pipeline/Jobs) and the composing-service views ADR 0011 already flags as
a future gap — this pass deliberately stopped at one route.
