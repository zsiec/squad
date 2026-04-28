---
id: FEAT-061
title: auto-postmortem dispatches a follow-up agent on long blocked or released claims
type: feature
priority: P3
area: learning
status: open
estimate: 4h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777345159
accepted_by: web
accepted_at: 1777345485
references: []
relates-to: []
blocked-by: []
epic: team-practices-as-mechanical-visibility
---

## Problem

Items closed as `blocked` or released without progress after a long
held claim represent the most expensive failure mode in the system —
hours of agent time with nothing durable to show. Today the lesson
dies in the chat thread: the blocker note in the item file is short,
chat scrolls past, and the next agent hitting the same wall has to
rediscover the dead-ends. A human team would write a postmortem; the
agent-realm equivalent must capture the lesson before the context
evaporates.

## Context

`internal/learning/` already has the artifact format (`.squad/learnings/<slug>.md`,
gotcha / pattern / dead-end kinds) and the propose → human approve →
auto-load flow. The skill `squad-handoff` already prescribes a
"surprised by" bullet on session pause. What's missing is the
trigger: nothing fires automatically when a claim ends in a way that
suggests durable lesson-capture is warranted.

The signal is in the ledger: claim `claimed_at`, `last_touch`, and
release/blocked event timestamps. A claim held >2h that ends in
`blocked` or `released` (without a paired `done`) is a high-signal
trigger.

## Acceptance criteria

- [ ] When `squad blocked <ID>` fires on a claim held >2h, the daemon
      dispatches a follow-up subagent under a new
      `superpowers:postmortem` role with the item file path, the
      thread chat history, and any handoff/session-log block as
      input.
- [ ] Same trigger when `squad release <ID> --outcome released` is
      called on a claim held >2h.
- [ ] The follow-up agent writes a `dead-end` learning artifact to
      `.squad/learnings/<slug>.md` (auto-slugged from item title +
      timestamp) with the standard structure: hypotheses tried,
      ruled-out causes, evidence collected, what to do differently
      next time. The artifact is in the propose state — surfaces
      to the operator for approval.
- [ ] Configurable threshold (`postmortem.long_block_seconds`,
      default 7200). Setting to 0 disables the trigger entirely.
- [ ] Test: simulate a long-held blocked claim, assert the
      postmortem agent gets dispatched and the learning artifact
      appears in the propose-state directory.
- [ ] Test: a short blocked claim (< threshold) does NOT dispatch.

## Notes

- The postmortem agent should explicitly avoid blame language —
  the artifact is a lesson, not a verdict. Build into the skill
  prompt: "describe what was tried, what was learned, what
  evidence ruled out which hypotheses; do not name agents in
  failure context."
- Lands last in the epic. The retro (FEAT-059) will surface
  whether long-block frequency justifies the auto-dispatch
  cost; if blocked claims are rare, this item may not be worth
  shipping in its current form.

## Resolution
(Filled in when status → done.)
