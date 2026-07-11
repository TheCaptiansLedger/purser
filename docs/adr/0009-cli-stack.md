# 0009. CLI Stack: Cobra, pterm, and Bubble Tea

Status: Accepted

## Context

Purser ships a CLI (`cmd/purser`, plus tooling like `cmd/seed`) alongside its
server component. Command parsing, one-shot styled output, and full
interactive screens are three different problems; picking one library and
bending it to cover all three produces either a command parser doing manual
flag work or a TUI framework abused for a single progress bar.

## Decision

- **Cobra** (`github.com/spf13/cobra`, already a dependency) is the only
  command/flag parsing framework. Every CLI entry point is a Cobra command;
  no hand-rolled `os.Args`/`flag` parsing in `cmd/**`.
- **pterm** (`github.com/pterm/pterm`) is used for styled, non-interactive or
  short-lived-interactive terminal output: spinners, progress bars, tables,
  colored status/success/error messages, and simple yes/no or single-value
  prompts. Use pterm when the command runs, prints/updates output, and exits.
- **Bubble Tea** (`github.com/charmbracelet/bubbletea`) is used for full
  interactive, stateful terminal UIs — multi-view screens, anything with
  its own event loop, keyboard-navigable lists/forms, or a screen that stays
  open and redraws in response to user input across multiple steps. Use
  Bubble Tea only when the interaction has enough state that pterm's
  single-pass prompts would need to be chained ad hoc to fake a screen.
- **Decision rule:** if the interaction can be described as "run a task, show
  progress, print a result," it's pterm. If it can be described as "a screen
  the user navigates around in," it's Bubble Tea. A command should not mix
  both for the same interaction — pick one per screen/command.
- Output written for machine consumption (`--json`, piped output, non-TTY)
  bypasses both libraries and writes plain structured output directly;
  pterm/Bubble Tea are for the human-attended terminal case only.

## Consequences

- Cobra commands stay thin wiring (flags → service call), consistent with
  the `cmd/**` "composition root" role described in
  [0001](0001-hexagonal-architecture.md) and the 50% coverage bar in
  [0003](0003-go-testing-standards.md).
- Contributors don't have to guess which of two capable-but-different
  terminal UI libraries to reach for — the decision rule above is meant to
  be mechanical, not a style preference re-litigated per PR.
- Adds two terminal UI dependencies to the module; both are scoped to
  `cmd/**` and must never be imported from `internal/**` or `pkg/**` (which
  have no business rendering a terminal UI).

## Self-Audit Checklist

1. Does any command in `cmd/**` parse flags/args without going through
   Cobra? If yes — fix it.
2. Does any `internal/**` or `pkg/**` package import `pterm` or
   `bubbletea`? If yes — that's a layering violation; terminal rendering
   belongs in `cmd/**` only.
3. For each interactive command added, does it match the decision rule (run
   task/show progress/print result → pterm; navigable screen with its own
   state → Bubble Tea)? If a command mixes both for one interaction, split
   it.
4. Does machine-consumable output (`--json`, non-TTY) still bypass pterm/
   Bubble Tea styling? If no — fix it.
