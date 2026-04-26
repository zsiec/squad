---
id: BUG-005
title: agents work items quietly — chat cadence guidance lives in deep tier + skill but never reaches the per-claim, per-milestone, per-commit moments where it would force action
type: bug
priority: P2
area: docs
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777238580
accepted_by: agent-bbf6
accepted_at: 1777238580
references: []
relates-to: []
blocked-by: []
---

## Problem
Observed across multiple agents in this session: agents claim items, grind
through them, and commit with little or no chat traffic in between. The
team loses visibility into root-cause chains, surprises, dead ends, and
learnings that would help peers route attention or pick up dropped
threads. Squad already has the verbs (`thinking`, `milestone`, `stuck`,
`fyi`, `ask`, `say`) and cadence rules, but the rules don't reach the
agent at the moments where they would force action.

## Context
Where guidance currently lives:

- `AGENTS.md` (fast tier, always loaded): chat is only mentioned in
  passing — `§3` step 1 doesn't mandate posting; `§8` anti-patterns omit
  "don't work silently."
- `docs/agents-deep.md` (deep tier): has a `## Chat cadence` section
  (lines 123–138) with the verb table and the canonical moments to post
  (on claim / direction change / AC complete / commit / surprise /
  blocker / session pause). But the deep tier only loads for ≥1d items,
  architectural calls, or skill auto-loads — most claims don't hit it.
- `squad:squad-chat-cadence` skill: declared in the plugin's available
  skills but auto-loads via `paths:` globs, not via claim/commit
  triggers. So an agent can claim, edit, test, commit, close — without
  the skill ever activating.

Why this matters:

- Peers can't react to a non-obvious finding mid-flight. (e.g. in this
  session BUG-002 root-cause was a 3-link chain through register.go →
  ensureRegistered → /api/agents that overlapped with BUG-001's per-repo
  claims work; without a `fyi`, agent-401f had no visibility.)
- Future sessions reading the chat archive can reconstruct the *what*
  from commits but not the *why* / dead-ends — exactly what the cadence
  rule is designed to capture.
- Humans watching the dashboard see opaque "agent is working" cells
  instead of a stream of typed updates that show progress and posture.

What's been tried: the deep tier already documents the cadence; the
skill already exists. Neither reaches the agent at the right moment.

## Acceptance criteria
- [ ] AGENTS.md (fast tier) gets a top-level chat-cadence section that
      lists the verbs and the canonical post-moments (claim, RED→GREEN,
      AC complete, surprise, blocker, commit, learning, done).
- [ ] AGENTS.md `§8 — Anti-patterns` adds an explicit
      "Don't work silently" / "Don't grind through an item with no chat
      between claim and done" line.
- [ ] At least one CLI seam emits a cadence nudge in-flow — e.g.
      `squad claim` reminds the agent to post a `thinking` with their
      one-sentence intent, and `squad done` reminds to post a learning
      if anything non-obvious surfaced. Output must be terse (one line),
      and suppressible (env var or flag) so it doesn't flood scripts.
- [ ] The `squad:squad-chat-cadence` skill auto-loads at `squad claim`
      time (or equivalent trigger), so the skill content reaches the
      agent's context regardless of which files they touch.
- [ ] Verification: a fresh session reading only AGENTS.md (no deep
      tier, no skill bodies) lands enough cadence guidance to post at
      the canonical moments. Either a manual walk-through or a fixture
      test against the rendered AGENTS.md content.
- [ ] Doc updates regenerate cleanly via `squad init` — i.e. the
      template (`AGENTS.md.tmpl`) is the source of truth, not the
      checked-in copy in this repo.

## Notes
- Don't bloat AGENTS.md. The fast tier earns its name; the cadence
  section should be one small block (verb table + 1-line cadence
  rule + anti-pattern bullet), not a wall of text.
- This is not "spam more chat." Skill description is correct: visibility
  into non-obvious state, not a change log. The fix is making the rule
  visible at the moment of action.
- Coordinate with the learning system: a `squad done` nudge to file a
  `squad learning propose` when something durable surfaced is the
  natural complement to chat cadence.
- Implementation hint: per-claim skill auto-load may not be supported
  by the plugin host today; if so, file a follow-up CHORE rather than
  block this item on it. The doc + CLI-nudge AC should land regardless.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
