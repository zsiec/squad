---
id: FEAT-042
title: 2h time-box async wakeup at 90m and 120m of held claim
type: feature
priority: P2
area: cmd/squad
status: open
estimate: 3h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309558
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: cadence-and-time-boxing-as-pacing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

`CLAUDE.md` and the `squad-time-boxing` skill specify a 2h cap on
exploratory work. Nothing wakes an agent at the cap, so the rule is
aspirational — long unsuccessful sessions slip past the boundary
unnoticed.

## Context

The async-wakeup machinery already exists for P0/P1 claim nudges:

- `cmd/squad/cadence_nudge.go` is the existing nudge entry point.
- `plugin/hooks/async_rewake.sh` is the wiring that dispatches a wake
  back into the session.
- `plugin/skills/squad-time-boxing/SKILL.md` is the documented rule the
  wakeups need to enforce.

Search how the existing P0/P1 claim nudge schedules itself off
`squad claim`; the 90m / 120m wakeups should follow the same pattern,
indexed by claim time, scoped to the held claim, and cancelled on
`done` / `release`.

## Acceptance criteria

- [ ] `squad claim` schedules an async wakeup at +90m and +120m relative
      to claim time.
- [ ] The 90m wakeup posts a `thinking`-prompt if no `milestone` has
      been posted in the last 30m.
- [ ] The 120m wakeup prompts handoff or split-and-park.
- [ ] Both wakeups cancel cleanly on `squad done` / `squad release`.
- [ ] Suppressible via the existing `SQUAD_NO_CADENCE_NUDGES=1` knob.

## Notes

Reuse the existing nudge dispatcher rather than introducing a parallel
scheduler. The 90m wakeup should be a no-op when a recent `milestone`
exists — the goal is to catch silent agents, not to spam noisy ones.

## Resolution

(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
