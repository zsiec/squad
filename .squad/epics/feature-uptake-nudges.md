---
spec: feature-uptake-nudges
status: open
parallelism: |
  Phase A foundation (item #1, config + Done() fallback) is the only required serialisation point —
  it unlocks #3 (scaffold config) which depends on the field existing. Phases B/C/D are independent
  of each other and of the rest of A: they touch cmd/squad/cadence_nudge.go (additive helpers) and
  cmd/squad/claim.go or done.go (one-line wiring), with no overlapping symbols. Item #2 (squad new
  template) is independent of everything except its own tests. Item #7 (skill prose) waits for the
  feature commits so the docs land alongside what they describe. Item #8 (dogfood walkthrough) is
  the integration gate and waits for everything except #4 (the evidence-requirement skill).
  Recommended dispatch: items 1, 2, 4, 5, 6 in parallel; #3 after #1; #7 after the feature items;
  #8 last as the integration test.
---

## Goal

Close the four gaps surfaced in the dogfood usage analysis (25 done items / 0 attestations / 0 learnings / 2 `ask` verbs) so the existing attestation, learning, and peer-chat features start producing data without adding new schemas, migrations, or hard gates.

## Child items

The 8 implementation tasks are filed as child items, each with `epic: feature-uptake-nudges` in their frontmatter. Acceptance criteria for each item are written so a fresh agent can execute the item without reading the spec.

## Anti-patterns to avoid during execution

- **No new tables, no schema migrations.** Every change reuses primitives that already exist.
- **No nudge becomes a gate.** The only blocking change is the existing `evidence_required` gate, and that's per-item opt-in (or repo-level default that operators set via config).
- **No per-nudge env vars.** `SQUAD_NO_CADENCE_NUDGES=1` covers all four nudges.
- **No bundling phases into one commit.** Each child item is one commit (or a small handful) so reverts are surgical.
