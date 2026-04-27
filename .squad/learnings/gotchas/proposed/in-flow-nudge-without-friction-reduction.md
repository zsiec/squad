---
id: gotcha-2026-04-27-in-flow-nudge-without-friction-reduction
kind: gotcha
slug: in-flow-nudge-without-friction-reduction
title: Done-time learning nudge fires reliably but follow-through is rare — propose-time friction outweighs the nudge's encouragement
area: cli-nudges
paths:
  - cmd/squad/cadence_nudge.go
  - cmd/squad/learning.go
  - plugin/skills/squad-done/SKILL.md
created: 2026-04-27
created_by: agent-bbf6
session: 
state: proposed
evidence: []
related_items: []
---

## Looks like

A one-line in-flow nudge — `tip: any gotcha worth filing? \`squad learning propose gotcha <slug>\` ...` — printed to stderr after `squad done` should be enough to drive learning capture, since it pairs the prompt with the moment of greatest information density (the close-out, when the surprise is freshest).

## Is

The nudge fires correctly and is read, but follow-through is rare because the *propose* step has friction the nudge does not address. To file a learning, the agent must (1) pick a kind, (2) author a kebab-case slug, (3) write a title, (4) pick an area tag, (5) edit the body to fill three sub-sections. That's ~30s of friction at a moment when the agent is trying to ship and pick up the next item.

Empirical evidence from the `feature-uptake-nudges` epic (2026-04-26):
- 8 epic items closed (`TASK-012` through `TASK-018` + `TASK-019`)
- 9 attestations recorded (mandatory gate, friction-light: one CLI invocation)
- 0 learnings filed during the work itself
- This learning, the first in the ledger, was only filed *because* the dogfood walkthrough explicitly carved out time to demonstrate the flow

Surprises that would have made good gotchas (e.g., agent-401f's BUG-017 fix silently breaking two tests via `bootClaimContext`'s shared traversal with `postRunHygiene`) ended up in `surprised_by` of session-handoff payloads instead of the learning corpus, where they are less indexable and only surfaced if a human reads the handoff.

## So

Two complementary directions, in order of likely leverage:

1. **Reduce propose-time friction.** A `squad learning quick "<one-line>"` shorthand that captures a free-form sentence into `proposed/` with auto-derived slug and area, leaving categorization for the human approval step. Lets an agent file a gotcha in 5 seconds at done-time without losing flow.

2. **Pull from already-captured signal.** Session-handoff payloads carry a `surprised_by` array that is *exactly* the gotcha shape. A `squad handoff --propose-learnings-from-surprises` flag could auto-stub one learning per `surprised_by` entry, again leaving categorization for human approval. Treats the handoff payload as the funnel rather than asking agents to file twice.

Until one of those exists, future agents reading this gotcha should treat the in-flow nudge as a *signal to handoff with `surprised_by` populated* rather than a signal to drop everything and run `squad learning propose`. The handoff payload is the realistic filing surface; the propose CLI is for human-mediated curation, not in-flow capture.
