# 0006. Commit Message Conventions

Status: Accepted

## Context

Commit message format was previously carried only in personal memory
(a "no `BREAKING CHANGE:` footer pre-1.0" rule and general Conventional
Commits usage), not in any doc in this repository. That gap was only
caught because it was asked about directly — nothing written down would
have surfaced it otherwise. This ADR pins the convention down so it can be
checked mechanically instead of re-derived from memory every time.

This ADR governs the message text only. The hard ban in `AGENTS.md` still
applies: an agent produces commit message text and stops — it never runs
`git commit`, `git push`, or `gh pr create`.

## Decision

### Subject line

```
<type>[optional scope][!]: <summary>
```

- `type` is one of the Conventional Commits types (`feat`, `fix`, `chore`,
  `docs`, `refactor`, `test`, `perf`, `build`, `ci`) — pick the one that
  matches what the commit actually does, not the broadest one that fits.
- `!` after the type/scope marks a breaking change (see
  [0005 breaking-change label](0005-github-issue-format.md) for the
  corresponding issue/PR label — pre-1.0, `!` plus the body explanation is
  used instead of a `BREAKING CHANGE:` footer).
- `summary` is under 80 characters total (including `type:` prefix), written
  in the imperative present tense as if finishing the sentence "If applied,
  this commit will ___": `add foo capabilities`, not `added foo
  capabilities` or `adds foo capabilities`.

### Body

One or two paragraphs. The body explains **why this code needs to exist**
— what feature it adds or what bug it fixes and what breaks or stays
missing without it — never a list of which files changed or a mechanical
description of the diff. If the reader can already tell what changed from
the diff itself (they always can), restating it in prose adds nothing;
the diff can't explain motivation, so that's the body's only job.

Bulleted lists are not used to describe the change in general. The one
exception: when a commit closes an issue that itself enumerated discrete
fixes or implementation items (a checklist-style issue), a bullet list may
enumerate which of those specific items this commit addresses — not a
generic list of touched files or functions.

### Footers

Standard Conventional Commits footers apply (`Co-authored-by:`,
`Refs:`, etc. as needed). On this project, issue linkage always uses one
of:

- `Closes: #<issue>` — this commit is what completes the referenced issue.
- `Part-Of: #<issue>` — this commit contributes to the referenced issue but
  does not complete it on its own (more commits are still needed).

Use `Closes` only when the commit genuinely finishes the issue's scope as
written; if in doubt, use `Part-Of` rather than closing prematurely — a
falsely-closed issue is harder to notice than one that stays open one
commit longer than strictly necessary.

## Consequences

- Every commit message is checkable against this ADR without asking the
  user to restate the convention: type/scope/breaking-marker present,
  summary length and tense, body is prose-only unless enumerating issue
  sub-items, footer present when an issue is being tracked.
- A commit not tied to any GitHub issue simply omits the `Closes`/`Part-Of`
  footer — it is not fabricated to satisfy the format.

## Self-Audit Checklist

Run before handing a commit message to the user:

1. Is the subject line under 80 characters, in `type(!): summary` form,
   with the summary in imperative present tense ("add," not "added" or
   "adds")?
2. Does the body explain *why* the change exists (feature/bug motivation),
   with no restatement of which files or functions changed?
3. Is the body prose-only, unless the commit closes an issue with an
   itemized checklist — in which case bullets may map to that issue's
   specific items?
4. If this commit is tied to a GitHub issue, does it use `Closes: #N` only
   when the issue's full scope is actually done, and `Part-Of: #N`
   otherwise?
5. Is this ADR itself up to date with what was just asked of it — if the
   user corrects this convention again, is the correction going into this
   file, not just into memory?
