# Version Control Workflow

## Branching Model

Day-to-day work targets `develop`. Releases happen when `develop` is merged into `main`.

```
feature/* ──┐
fix/*    ───┼──► develop ──► main (release)
chore/*  ──┘
```

- `develop` is the default branch — all feature/fix/chore PRs target it
- `main` is the release branch — only receives PRs from `develop`
- Merging `develop` → `main` triggers semantic-release, container build, and release notes

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
- Body should contain a small paragraph on the WHY not the WHAT. WHY is this code going in? what does it add for the user or project? 
- footers should include breaking changes and a resolves: issue #xxx or Part-Of: issue #xxx if not fully resolving issue

```

## Pull Requests

- PR title and body should mimimc the commits in the delta
- If multiple commits, use the issue title as the pr title
- Keep PRs focused — one feature or fix per PR
- All checks must pass before merge

## Merging

GitHub's web merge operations (squash, rebase, create merge commit) all produce commits signed by GitHub's CA key, not yours. To keep every commit — including the merge commit — signed with your GPG key, merge locally after CI passes and the PR is approved:

```bash
git fetch && git checkout develop && git pull && git merge --no-ff origin/<branch> && git push
```

GitHub auto-closes the PR when it detects the branch is merged via the commit graph.

Never use GitHub's merge button — it will strip or replace your signature regardless of which strategy is chosen.
