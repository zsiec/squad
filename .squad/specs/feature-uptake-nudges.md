---
title: Drive agent uptake of attestations, learnings, peer chat, and milestones
motivation: |
  Dogfood data over the first 25 closed items shows three load-bearing squad features at zero or near-zero usage:
  the attestation ledger has 0 rows, the learning corpus has 0 entries, and `squad ask` peer-to-peer was used twice
  across the entire session. The plumbing is fully built — `Done()` already enforces `evidence_required`, the
  Stop hook prompts for learnings, the chat-cadence skill names every verb. What's missing is the structural pull
  that would make agents reach for these features in-flow. This spec closes the gap with one config-level default
  (lighting up the existing attestation gate) and three new in-flow nudges (modeled on the working cadence nudge),
  none of which add new tables, schema migrations, or blocking gates beyond the per-item opt-in that already exists.
acceptance:
  - "`squad new BUG ...` writes `evidence_required: [test]` into the item template by default; CHORE/DEBT/BET stay at `[]`."
  - "`squad init` writes `defaults.evidence_required: [test]` into the scaffolded `.squad/config.yaml`."
  - "`Done()` falls back to `cfg.Defaults.EvidenceRequired` when an item has no per-item field set, so the existing gate fires repo-wide once a maintainer opts in."
  - "`squad claim` on a P0/P1 or `risk: high` item prints a one-line stderr nudge pointing at `squad ask @<peer>`, suppressible via `SQUAD_NO_CADENCE_NUDGES=1`."
  - "`squad claim` on an item with ≥2 acceptance-criteria boxes prints a one-line stderr nudge that names the AC count and recommends ~that many `squad milestone` posts."
  - "`squad done` on a `bug`-typed item prints a stronger one-line stderr nudge specifically pointing at `squad learning propose gotcha`."
  - "`plugin/skills/squad-evidence-requirement/SKILL.md` mandates a `squad attest` invocation alongside the chat paste, with concrete command examples for `--kind test`, `--kind review`, `--kind manual`."
  - "Dogfood walkthrough on one real open item demonstrates the new flow end-to-end: claim with nudges fired, milestone posts at each AC, `squad attest --kind test` recorded, `squad done` clearing the gate without `--force`, optional `squad learning propose` filed."
non_goals:
  - "New schema or migrations. Every change reuses `attestations`, `messages`, and `items` as they exist today."
  - "Promoting any nudge to a blocking gate. The only blocking change is the existing `evidence_required` gate, and it only fires for items that opt in (per-item field or repo-level default)."
  - "Per-nudge env-var knobs. One global silence (`SQUAD_NO_CADENCE_NUDGES`) is sufficient."
  - "Auto-filing learnings. The Stop hook + nudge guide the agent to file by hand; the binary does not invent learnings."
  - "Forcing `squad ask` on every claim. The nudge fires only on P0/P1 or `risk: high` work."
integration:
  - "internal/config/ — Defaults gains an EvidenceRequired []string field"
  - "internal/items/ — new.go templates the field for bug/feat/task; CountAC helper consumed by the milestone nudge"
  - "internal/scaffold/ — generated config.yaml includes the default"
  - "cmd/squad/cadence_nudge.go — gains printSecondOpinionNudge, printMilestoneTargetNudge, and a type-aware variant of printCadenceNudge"
  - "cmd/squad/done.go — DoneArgs gains DefaultEvidenceRequired, populated by the cobra wrapper from config"
  - "cmd/squad/claim.go — wires the new nudges after success"
  - "plugin/skills/squad-evidence-requirement, squad-done, squad-loop, squad-chat-cadence — text edits to align prose with the new in-flow signals"
---

## Background

The first 25 dogfood items split as 16 bugs, 2 features, 7 tasks, 1 chore. Across all of them: 0 attestation rows, 0 learning artifacts, 14 `@`-mentions, 2 `ask` verbs (both from the same agent to the same peer). Item threads averaged 3.4 messages — almost entirely the lifecycle triplet (`claim` / single `thinking` / `done`). The middle of the work was silent.

The features are not missing. `attestations` is a STRICT table with hash dedup, `EvidenceMissingError` already blocks `squad done` when an item declares `evidence_required`, the Stop hook already detects non-trivial diffs and prompts for learning proposals. What never connects is the *trigger* for the agent to use any of it: skills tell agents *how*, not *when*; the Stop hook fires too late in multi-item sessions; and items don't declare `evidence_required` because the template doesn't include the field.

The fix is small. Default the field on at the template + config layer. Add three one-line stderr nudges modeled exactly on `printCadenceNudge` (which has demonstrably altered observed agent behavior — every claim/done in the dogfood data has the cadence-nudge timing-fingerprint). Nudges are deliberately nags rather than gates: the per-AC milestone nudge could be too noisy for some workflows, and we want a one-env-var escape hatch rather than a discovery process to suppress it.

The full implementation plan is at `docs/plans/2026-04-26-squad-feature-uptake-nudges.md` (gitignored locally).

## Reading the success bar

Re-running the dogfood query a week after rollout should show:

| Metric | Before | Target |
|---|---|---|
| Attestations recorded | 0 | ≥1 per non-trivial close-out |
| Learnings proposed | 0 | ≥3 across closed bugs |
| `squad ask` peer-to-peer | 2 | ≥5 |
| `milestone` posts on multi-AC items | <1 | ~AC-count |

If the numbers don't move, nudges aren't strong enough — escalate one or more from "nudge" to "gate" (e.g., make `squad done` block on a non-test attestation for `risk: high` items).
