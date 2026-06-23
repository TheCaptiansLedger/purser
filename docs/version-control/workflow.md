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

- Subject line under 72 characters
- Present tense, imperative mood (`add` not `added`, `fix` not `fixed`)
- No period at the end of the subject line
- Do not add Co-Authored-By AI attribution
- Body should contain a small paragraph on the WHY not the WHAT. WHY is this code going in? what does it add for the user or project? 
- footers must use GitHub's exact auto-close syntax — `Closes #xxx` or `Resolves #xxx` (no "issue" between the keyword and the number; that word breaks GitHub's parser and prevents auto-close)
- use `Part-Of: #xxx` when the commit only partially resolves an issue

```

## commits

At the end of a session, present the suggested commit message and a summary of changes. Do NOT run `git commit` — the user commits manually.
