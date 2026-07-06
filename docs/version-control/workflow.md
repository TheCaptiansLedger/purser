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
```

| Field | Values |
|---|---|
| `type` | `feat` \| `fix` \| `chore` \| `docs` \| `refactor` \| `test` \| `ci` |
| `scope` | optional; name the sub-system (e.g. `stashdb`, `api`, `ui`, `db`, `config`) |

Rules:

- Subject line under 80 characters
- Present tense, imperative mood (`add` not `added`, `fix` not `fixed`)
- No period at the end of the subject line
- No commas in the subject line — if you reach for a comma it is too long or trying to say two things
- Do not add Co-Authored-By AI attribution
- Body explains the WHY: what does this change enable or fix? Not implementation mechanics, not renamed files, not a list of what changed
- Footers: `Closes: #xxx` to close an issue, `Part-Of: #xxx` when the commit only partially resolves an issue

```

## commits

At the end of a session, present the suggested commit message and a summary of changes. Do NOT run `git commit` — the user commits manually.
