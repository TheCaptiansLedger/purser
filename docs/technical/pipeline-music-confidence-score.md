# Music `ConfidenceScore` (M8)

Turns [pipeline-music-identifier.md](pipeline-music-identifier.md) (M7)'s
tiered, signaled candidates into an actual score. This is the direct fix
for the convergence bug described in
[0025](../adr/0025-music-identification-confidence-scoring.md)'s Context —
the reason this whole design effort started. See
[music-pipeline-milestones.md](music-pipeline-milestones.md) for where M8
sits in the build order.

## Scope

**In scope:** the per-tier scoring formulas, the release-group ambiguity
cap, and the fixture-driven differentiation test suite required before
this is trusted for auto-import.

**Explicitly out of scope:** anything M7 already owns (candidate
discovery, tier assignment, raw signal computation) and anything M9 owns
(comparing the winning score to the configured auto-import threshold,
persisting the result).

## The port is batch-shaped, not per-candidate — a correction to 0024

[0024](../adr/0024-pipeline-core.md) describes `ConfidenceScore` as
"given a `Fingerprint` and a candidate, return a score" — one candidate at
a time. But the ambiguity cap (below) needs to compare the top candidate
against its runner-up, which a strictly per-candidate call can't see. This
is the third capability 0024 described as per-file/per-candidate that
turned out to need batch shape once the mechanics were worked out —
grouping (M3) and fingerprinting (M4) both went the same way; worth
naming as a recurring pattern in this build, not a one-off surprise.

```
ConfidenceScore(ctx, fingerprint *Fingerprint, candidates []MatchCandidate) ([]MatchCandidate, error)
```

Takes M7's whole list (`Tier`/`Signals` populated, `Score` unset), returns
it with `Score` populated on every candidate — ambiguity already folded
into whichever one ends up on top.

## Per-tier formulas

### `direct_id`

Flat `0.98`. M7 already short-circuits to a single candidate for this
tier — nothing to average, nothing to differentiate.

### `unique_id`

```
base := 0.80                      // barcode OR ISRC consensus alone
if barcodeMatch && isrcConsensus {
    base = 0.90                   // both agree on the same release
}
corroboration := average(evaluated fuzzy signals: track_count_match, title_set_overlap, duration_match)
score := base + corroboration*(0.95-base)   // capped below direct_id's 0.98
```

The corroboration term is a continuous average, not a boolean — two
barcode matches with different levels of fuzzy corroboration still
differentiate, which is the whole point.

### `fuzzy` — the actual fix

```
signals := {name_fuzzy_score: 0.30, track_count_match: 0.20, title_set_overlap: 0.30, duration_match: 0.20}
// (+ acoustic_agreement folded in here too, see "AcoustID's dual role" below, if it ran)

sumWeight, sumWeighted := 0, 0
for key, weight := range signals {
    if evaluable(key) {           // real data existed on BOTH our side and the candidate's side
        sumWeight += weight
        sumWeighted += value(key) * weight
    }
}
rawAvg := sumWeighted / sumWeight // only over what was actually evaluated
score := 0.45 + rawAvg*(0.75-0.45)
```

**This is the mechanism that fixes v1's bug.** `sumWeight` only ever
includes signals that had real data to compare — a group with no barcode
tag doesn't drag every candidate toward the same low number the way
dividing by the weight of *every possible* signal did in v1. Starting
weights (0.30/0.20/0.30/0.20) are a proposal to validate against the
fixture suite below, not asserted correct.

### `acoustic` — a dual role, not a single formula

AcoustID plays two different roles depending on what else is available,
and conflating them would silently break the "≤0.60 unless corroborated"
rule:

1. **Sole evidence** (candidate doesn't otherwise qualify for `fuzzy`):
   scored on `acoustic_agreement` alone, mapped into a capped band
   (`≤0.60`). Audio fingerprinting alone is less trustworthy than
   corroborated tags — cover versions and remasters can fool it — so it
   gets a structurally lower ceiling even at maximum confidence, not just
   a lower starting point.
2. **Corroboration** (candidate already qualifies for `fuzzy` on its own
   signals): `acoustic_agreement` becomes one more term in the `fuzzy`
   weighted average above — a bonus within that band, not a separate tier.
3. **Contradiction**: if AcoustID confidently resolves to a *different*
   release than the one being scored, that's not merely "one signal
   scored zero" — it's active evidence against the candidate. Applies an
   explicit demotion multiplier (e.g. `×0.5`, a starting number) on top of
   whatever the weighted average produced. Confident contradiction is
   stronger evidence than absent corroboration and needs to be treated
   that way, not averaged in as just another low number.

## Release-group ambiguity: a fixed low ceiling, not the real threshold

The auto-import threshold is shared-service/config-owned
([0024](../adr/0024-pipeline-core.md)); `ConfidenceScore` is Music-owned.
Rather than injecting that config value into Music's scoring code:

1. Group scored candidates by **release group** (not release/edition —
   MB's release→release-group parent relationship).
2. Multiple releases within the *same* release-group cluster: not
   ambiguity, but not a no-op either — resolved deterministically to a
   single representative, and every other same-cluster candidate that
   would tie or beat it is pushed strictly below it by a small fixed
   margin (`0.001`). This matters beyond bookkeeping: near-duplicate
   editions of the correct release group routinely produce an *exact* raw
   score tie (nothing about tags/durations distinguishes a 1980 pressing
   from a 2004 reissue), and a downstream "take the single highest Score"
   consumer ([pipeline-music-persist.md](pipeline-music-persist.md)'s
   `DecisionService`) needs that tie broken here, not left for it to
   resolve arbitrarily. `IsDefault` itself is unreachable at this
   layer — nothing is persisted yet at scoring time, so there's no local
   `music.Release` row to read it from — so the representative is instead
   preferred by MusicBrainz's own `Status == "Official"` (over
   `Promotion`/`Bootleg`/`Pseudo-Release`), falling back to highest raw
   score, falling back to first-seen for full determinism when neither
   distinguishes them.
3. Multiple *different* release-group clusters whose (now tie-broken,
   unique) top scores are within a margin of each other (starting point:
   `0.05`–`0.10`, needs fixture validation): real ambiguity. Clamp the
   winning score to a fixed ceiling — `0.50`, comfortably below `fuzzy`'s
   entire band — regardless of what the per-candidate formula computed.

The fixed ceiling means this works for any sane threshold configuration
without Music's scoring code needing to know what that threshold actually
is — keeps the content-type-owned/shared-service-owned boundary from
[0024](../adr/0024-pipeline-core.md) intact. Resolving same-cluster ties
here, rather than leaving them to whatever `DecisionService` does with a
tied `max(Score)`, keeps that same boundary intact too — M9 stays a dumb
consumer of one number per candidate, never needing edition-preference
logic of its own.

## Signal key contract (M7 → M8)

`barcode_match`, `isrc_consensus_fraction`, `name_fuzzy_score`,
`track_count_match`, `title_set_overlap`, `duration_match`,
`acoustic_agreement` — all `0.0`–`1.0`, **absent** (not zero) when M7
couldn't evaluate them. The absent/zero distinction is load-bearing — see
the `fuzzy` formula above.

## Fixture suite (required before this is trusted)

Per [0025](../adr/0025-music-identification-confidence-scoring.md)'s
self-audit item 1:

- Perfect tags + barcode (`unique_id`, both-agree base).
- Missing barcode, present ISRC (`unique_id`, ISRC-only base).
- Correct tracks, wrong/missing album tag (`fuzzy`, exercises the
  evaluable-signals-only normalization directly).
- Multi-disc release (exercises `(disc, track)`-keyed title-set/duration
  comparison from M4/M7).
- A Various Artists compilation.
- Two near-duplicate editions of the same release group (exercises the
  edition-vs-release-group ambiguity split — must **not** trigger the
  cap).
- Two different release groups scoring close together (must trigger the
  cap).
- AcoustID contradicts otherwise-strong tag signals (exercises the
  demotion multiplier).

**The differentiation assertion** — the actual regression guard against
this bug recurring: a "strong fuzzy match" fixture must score at least a
fixed margin (starting point: `0.15`) above a "weak/wrong fuzzy match"
fixture run through the *same* code path. Without this explicit assertion,
a future change could reintroduce convergent scoring and every other test
would still pass.
