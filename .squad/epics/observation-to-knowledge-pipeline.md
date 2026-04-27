---
title: Observation to knowledge pipeline
spec: agent-team-management-surface
status: open
parallelism: parallel
dependencies:
  - Refinement and contract hardening
intake_session: intake-20260427-44256e4424c4
---

## Failure mode

Agents emit observations constantly — handoff "surprised by" bullets,
doctor drift findings, code-reviewer rejection rationales — and almost
none of it survives the session. An audit of the live learning ledger
turned up exactly one proposed learning, and that learning was itself
a meta-observation noting this exact friction: the conversion from
"thing an agent noticed" to "artifact future agents will read" is
manual, optional, and therefore skipped.

The three biggest leaks today:

- The squad-handoff skill *requires* a "surprised by" bullet on every
  sign-off. Those bullets land in chat, age out of the visible window,
  and never become a `learning_quick` or a CLAUDE.md edit.
- `squad doctor` detects schema drift, malformed items, and stale
  claims. Each finding is a perfect candidate for `learning_propose`,
  with the diagnostic context already in hand. Today the finding is
  printed and forgotten.
- `superpowers:code-reviewer` rejections carry the highest-signal
  rationale in the system — a peer agent explaining why a change was
  wrong. None of that is mined.

## What this epic ships

Four items convert the three observation channels into durable
artifacts, plus close the last-mile gap between "approved learning"
and "CLAUDE.md PR":

- **FEAT-038** — squad-handoff skill prompts `learning_quick` per
  surprise bullet. Body pre-filled so the cost to file is one keypress.
- **FEAT-039** — `squad doctor` emits a `learning_propose` per finding
  kind, debounced per repo per 7 days. `--no-learnings` for operators
  who want the pure diagnostic.
- **FEAT-040** — `squad attest --kind review` with non-zero exit
  captures the rejection text and persists it as a `gotcha` learning.
  Repeat rejections on the same item get tagged `second-round`.
- **FEAT-041** — Audit (and if necessary, finish wiring) the
  `squad_learning_agents_md_*` MCP tools so an approved learning
  produces a real CLAUDE.md PR via `gh pr create`, not just a row in
  the suggestions table.

Together these close the loop: observation → propose → approve →
artifact merged.

## Why this depends on Epic B

The handoff-skill changes in FEAT-038 only matter if the bundle bodies
written by the skill are durable. The data-write path that BUG-028
hardens (the bundle commit/persist contract) is the same path that
`learning_quick --body` will exercise. Shipping FEAT-038 before
BUG-028 means new prompts feed an unreliable writer, and the friction
the epic is trying to remove just relocates.

The other three items are orthogonal to Epic B mechanically, but the
epic is treated as a unit: ship the handoff prompt and the writer
together so agents see the new cadence at the same moment the writer
becomes trustworthy.
