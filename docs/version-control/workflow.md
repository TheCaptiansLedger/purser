# Version Control Workflow

## Branching Model

Day-to-day work targets `develop`. Releases happen when `develop` is merged into `main`.

```
feature/*   ──┐
fix/*       ──┤
refactor/*  ──┼──► develop ──► main (release)
chore/*     ──┘
```

- `develop` is the default branch — all feature/fix/chore PRs target it
- `main` is the release branch — only receives PRs from `develop`
- Merging `develop` → `main` triggers semantic-release, container build, and release notes

## Branch Naming

```
feature/<number>-short-description    # new features tied to an issue
fix/<number>-short-description        # bug fixes tied to an issue
refactor/<number>-short-description   # refactors tied to an issue
chore/short-description               # maintenance, deps, config (no issue required)
```

## Commit Format

Conventional Commits:

```
type(scope): description

Paragraph 1 — why this matters.

Paragraph 2 — one supporting sentence, if needed.

Part-Of: #xxx
Closes: #xxx
```

| Field | Values |
|---|---|
| `type` | `feat` \| `fix` \| `chore` \| `docs` \| `refactor` \| `test` \| `ci` |
| `scope` | optional; name the sub-system (e.g. `stashdb`, `api`, `ui`, `db`, `config`) |

**Subject line**:
- Under 80 characters
- Names the capability or problem, not the files changed
- Present tense, imperative mood (`add` not `added`, `fix` not `fixed`)
- No period at the end
- No commas — if you reach for one it is too long or saying two things

**Body**:
- Two short paragraphs maximum
- Paragraph 1: one or two sentences stating the problem or gap this closes — what couldn't happen before?
- Paragraph 2: one sentence of supporting context or caveat, only if there is something meaningful to add
- No bullets, no section headers, no file names, no list of what changed

**Footers**: `Part-Of: #xxx` and/or `Closes: #xxx` on separate lines.

**Example**:

```
feat(adapter): add grouping capability to music module

Albums cannot be identified file-by-file; correct identification requires
all tracks from a folder to be evaluated together as a unit. This adds the
grouping layer that makes album-level identification possible.

We add a MusicGroupQueueWriter stub writer to persist entries so that we
can verify grouping through the API. Issue #406 will fully implement this.

Part-Of: #376
Closes: #400
```

## Commits

At the end of a session, present the suggested commit message. Do NOT run `git commit` — the user commits manually.
