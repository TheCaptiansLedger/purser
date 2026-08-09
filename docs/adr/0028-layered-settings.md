# 0028. Layered Settings: DB-Backed Runtime Overrides Beneath YAML/Env

Status: Accepted

## Context

[0010](0010-configuration.md) resolves configuration through three layers —
explicit Cobra flag > environment variable > `ops/purser.yaml`/`--config` >
code-defined default — and nothing else. There is no fourth layer, no way
to change a value without editing a file or restarting the process, and no
`ConfigService` of any kind exists yet (`internal/config.Load()` is
read-only; nothing serializes a `Config` back out). The planned web
Settings UI (`/settings/config`) needs operators to edit *some* values from
a browser and have them take effect immediately, while still letting an
IaaC-style operator lock any of those same values down via env var or
`purser.yaml` — the UI must never be able to override what the operator
explicitly set.

A pre-reset version of this codebase (before the `chore!: reset codebase`
commit `8ae1ac2`; see `internal/app/config/service.go` and
`internal/config/config.go` at commit `85fca8d`) solved a materially similar
problem: a `ConfigService` merging "operator config (env/yaml, locked)"
over "DB-stored runtime settings" over "built-in default," with a
`computeLockedKeys()` helper that scanned a file-only Viper instance plus
`os.LookupEnv` to determine which keys were operator-set. That code predates
the current `internal/config` package and the current `Config` struct shape
entirely — it is useful prior art for the shape of the problem, not code to
resurrect as-is.

Separately: Viper's own documented precedence order is *explicit `Set()` >
flag > env > config file > key/value store > default* — Viper already
reserves a conceptual slot for exactly this ("key/value store," originally
meant for etcd/Consul via Viper's remote-provider package). This ADR occupies
that same conceptual slot without adopting Viper's actual remote-provider
feature, which targets remote KV systems and pulls in dependencies
(etcd/Consul clients, `crypt`) this project has no other use for — a
generic `datastore.Datastore` document is not that.

## Decision

### Precedence

```
CLI flag > environment variable > YAML file > DB-stored override > code default
```

This extends [0010](0010-configuration.md)'s chain with exactly one new
layer, inserted between file and default. [0010](0010-configuration.md)'s
own three-layer mechanism is unchanged and remains the source of truth for
flag/env/yaml/default resolution — this ADR is additive, not a
replacement.

### Locked keys: two distinct reasons, both reject writes

A key is **operator-locked** if an environment variable or an explicit
`ops/purser.yaml` entry set it — computed the same way the pre-reset
`computeLockedKeys()` did: a file-only Viper instance's `IsSet` plus
`os.LookupEnv` against the key's `PURSER_<NESTED>_<KEY>` name. `UpdateSettings`
rejects writes to an operator-locked key.

A key is **bootstrap-locked**, unconditionally, regardless of whether an
operator set it, if it's needed to open the datastore in the first place:
`server.*`, `database.*`, `telemetry.*`, `log.*`, `paths.*`. These can never
appear in the DB-stored override layer — there is no chicken-and-egg
workaround for "the DB connection settings live in the DB." Bootstrap keys
are excluded from the DB-overlay key set entirely, not merely rejected at
write time, so they never round-trip through `Setting` at all.

`GetSettings` reports both the boolean `locked` flag and which reason
applies (`operator` vs. `bootstrap`), since the UI's messaging differs: an
operator-locked value says "set via env/yaml"; a bootstrap-locked value says
"requires a restart, never DB-editable."

### `Setting` domain + persistence: reuse `Repository[T]`, no new generic

A `Setting` is `{Key, Value string}`, keyed by `Key` — the same dotted
namespace [0010](0010-configuration.md) already uses for
`mapstructure`/Viper (e.g. `pipeline.confidence_threshold`,
`sources.stashdb.enabled`), just dot- instead of underscore-joined and
without the `PURSER_` prefix. This is a natural key, not a generated
UUIDv7 — the key *is* the identity, the same reasoning
[0019](0019-tag-identity-and-get-or-create.md) applies to why Tag's
identity isn't a bare generated ID, simpler here because there's no
composite tuple to reserve. `Value` is stored **JSON-encoded**, not a bare
string, so scalars, slices, and nested maps (e.g. a module's `roots []string`)
round-trip correctly through the same merge mechanism described below.

This is exactly [0012](0012-datastore-persistence.md)'s single-ID,
unfiltered shape — `internal/adapters/store.Repository[T]` is reused
directly, a ~15-line wrapper package (`internal/adapters/store/setting`),
zero new `Datastore`/migration work, consistent with 0012's own
"collapse it to the thin-wrapper pattern" self-audit rule. New port:
`ports.SettingsRepository`.

### Merge mechanism: gate `viper.Set` on the lock check, never call it unconditionally

After [0010](0010-configuration.md)'s existing `Load()` fully resolves
flag/env/yaml/default (unchanged — this is what makes the database itself
reachable), a second pass:

1. Computes the locked-key set (operator-locked ∪ bootstrap-locked).
2. Reads every `Setting` from `SettingsRepository`.
3. For each `Setting` whose key is **not** in the locked set, JSON-decodes
   its value and calls `viper.Set(key, value)`, then re-runs
   `viper.Unmarshal` into the final `Config`.
4. Re-validates via the existing `Config.Validate()`.

`viper.Set` is Viper's own highest-precedence write — safe here only
*because* step 3 gates it on step 1 first. Calling it unconditionally would
let a DB value silently outrank an operator's env/yaml value, which is the
one failure mode this whole ADR exists to prevent; this is called out
explicitly rather than left implicit, since it's the easiest part of this
design to get backwards.

Per-key **source** (`default`/`db`/`yaml`/`env`/`flag`) is computed
alongside this pass — flag-`Set`/env-lookup/file-`IsSet`/DB-presence checked
in that order, first match wins — and exposed by `GetSettings` for the UI's
provenance display. Viper itself does not expose "which layer resolved
this key" natively; this bookkeeping is this ADR's own addition, not a
Viper feature.

### Secret masking

Any field already treated as sensitive by convention (API keys, passwords,
DSNs) never round-trips in plaintext through `GetSettings`. The response
shows a fixed placeholder for a set secret and empty for an unset one.
`UpdateSettings` on a secret field is write-only: a request replaces the
stored value; there is no way to read the previous value back through the
API.

### Runtime effect without restart

Only the DB-overlay subset (the keys covered by steps 1–4 above — pipeline
thresholds/templates, module enabled/roots, source enable+API keys,
AfterDark provider priority, Prowlarr/QBittorrent/SABnzbd config) needs to
change live. `UpdateSettings` re-runs the merge pass and atomically swaps
in a refreshed snapshot after a successful write; bootstrap keys are exempt
by construction (they're never in this subset) and remain restart-only,
matching today's [0010](0010-configuration.md) behavior exactly.

## Consequences

- **Config resolution becomes two-phase.** [0010](0010-configuration.md)'s
  `Load()` alone no longer represents "the final config" for the
  DB-overlay subset — a second entry point (e.g.
  `ApplyOverrides(ctx, repo ports.SettingsRepository) error`) must run
  after the datastore adapter exists, called from `cmd/purser`'s
  composition root. This is a real ordering dependency to get right:
  the datastore itself must be constructed from the *bootstrap-only*
  `Load()` result before `ApplyOverrides` can run against it.
- **Some existing consumers of `internal/config.Config` need to become
  live-accessor consumers instead of one-shot values**, specifically for
  fields in the DB-overlay subset, if they're expected to reflect a
  `UpdateSettings` call without a restart. Which concrete consumers migrate
  is an implementation-task decision, not dictated field-by-field here.
- `ops/purser.yaml`/`.env.example` remain, unchanged from
  [0010](0010-configuration.md), documentation of available keys — the
  `Setting` store is a third source layered underneath them, never a
  replacement for that documentation.
- No cross-key write atomicity: a multi-key `UpdateSettings` call persists
  each `Setting` document independently, with no rollback if the process
  crashes mid-batch. This is the same class of accepted risk
  [0012](0012-datastore-persistence.md) already states plainly ("no
  DB-enforced referential integrity... a real place... there is no
  database-level safety net"), extended to this new collection rather than
  a new kind of risk.
- The shared error-mapping helper ([0011](0011-api-design.md)) needs one
  addition: a new sentinel (`ports.ErrLocked`) mapped to
  `connect.CodeFailedPrecondition` — the closest existing code to "rejected
  because of the system's current state," not `PermissionDenied` (no
  authorization concept exists here) and not `InvalidArgument` (the request
  is well-formed, just inapplicable to a locked key).

### Rejected alternatives

- **Viper's remote-provider (`viper.AddRemoteProvider`) KV feature.**
  Rejected: it targets etcd/Consul specifically and pulls in dependencies
  (`crypt`, etcd client) with no other use in this project. A local
  `datastore.Datastore` document needs none of that.
- **Storing the whole `Config` as one JSON blob per environment instead of
  per-key `Setting` rows.** Rejected: per-key locking and per-key source
  provenance are both required by `GetSettings`/`UpdateSettings`; a single
  blob would still need per-key metadata alongside it to answer "is *this*
  field locked," at which point per-key rows lose nothing and let
  `Repository[T]` be reused as-is instead of inventing a second shape.
- **Treating "restart required" as a UI-only convention for bootstrap keys**
  rather than excluding them from the DB-overlay key set structurally.
  Rejected: doesn't address the actual chicken-and-egg problem (resolving
  `database.*` requires the bootstrap layers to already be fully resolved,
  before any `SettingsRepository` can exist) — the exclusion has to be
  enforced by the merge pass itself, not left to client-side discipline.

## Self-Audit Checklist

1. Does any bootstrap key (`server.*`, `database.*`, `telemetry.*`,
   `log.*`, `paths.*`) ever get written to or read from the `Setting`
   store? If yes — fix it; these stay env/yaml/default only, permanently.
2. Does `UpdateSettings`/`ResetSetting` ever write before checking both
   lock reasons (operator-locked *and* bootstrap-locked)? If yes — fix it.
3. Does `GetSettings` return a secret field's real value instead of a
   masked placeholder? If yes — fix it.
4. Does the merge pass call `viper.Set` for any key without first
   confirming that key is unlocked? If yes — fix it immediately; this is
   the exact bug that lets a DB value silently outrank an operator's
   env/yaml value.
5. Does a new DB-overlay-eligible config field get added to
   `internal/config` without also being added to the overlay's key set (so
   it displays as editable in the UI but a write silently has no effect)?
   If yes — fix it.
6. Is any `Setting.Value` stored as anything other than JSON-encoded (so a
   slice/map field fails to merge correctly via `mapstructure`)? If yes —
   fix it.
