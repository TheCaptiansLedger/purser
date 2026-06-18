# Version Control Workflow

## Branch Naming

```
feature/<number>-short-description    # new features tied to an issue
fix/<number>-short-description        # bug fixes tied to an issue
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
- Body should contain a small paragraph on the why, not the what
- footers should include breaking changes and a resolves: issue #xxx or Part-Of: issue #xxx if not fully resolving issue

```

## Pull Requests

- PR title and body should mimimc the commits in the delta
- If multiple commits, use the issue title as the pr title
- Keep PRs focused — one feature or fix per PR
- All checks must pass before merge
