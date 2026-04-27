---
id: FEAT-042
title: 2h time-box async wakeup at 90m and 120m of held claim
type: feature
priority: P2
area: cmd/squad
status: done
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

**Phased delivery:** this commit is the tick-driven phase. FEAT-051
captures the listener-pollMailbox integration that fulfills the "true
async" half of AC bullet 1 (catches agents who go silent for 2h with
no Bash boundary). Reviewer flagged the gap; the marker-on-claim
dedupe in `maybePrintTimeBoxNudge` lets both paths fire safely.

**Schema:**
- `internal/store/migrations/012_claim_timebox_nudges.sql`:
  `ALTER TABLE claims ADD COLUMN nudged_90m_at INTEGER` and
  `nudged_120m_at INTEGER`. Both nullable — NULL means unfired.
- `internal/store/migrate.go`: v12 marker probe. Three pre-existing
  version-count assertions bumped 11→12.

**Nudge logic (`cmd/squad/cadence_nudge.go`):**
- New `timeBoxNudgeText(claimAge, sinceLastMilestone)` pure helper
  returns the per-threshold copy or "" when silenced/below threshold.
- 90m branch: prompts `squad thinking` only when `sinceLastMilestone
  >= 30m`; quiet otherwise. Goal is to catch silent agents, not nag
  noisy ones.
- 120m branch: prompts `squad handoff` and points at the
  `squad-time-boxing` skill so an agent who hasn't loaded the skill
  knows where to read about split-and-park.
- `maybePrintTimeBoxNudge` looks up the held claim, computes age,
  consults the marker columns, fires + stamps when warranted.
  120m takes priority — at the hard cap we don't also re-emit the
  90m nudge.
- Marker-on-fire (not on-cross): if the 90m branch is silenced by a
  recent milestone, the column stays NULL so a later tick (after the
  silence window expires) still has a chance to fire — "fire when
  silent at 90m," not "fire exactly once at the 90m mark."

**Cancellation on done/release:** falls out of the data model. The
claims row is deleted on release/done (`internal/claims/release.go`
and `done.go`); the marker columns vanish with the row, so a re-claim
of the same item gets a fresh INSERT with markers back to NULL.

**Wiring:** `cmd/squad/tick.go` calls `maybePrintTimeBoxNudge` after
`maybePrintStaleChatNudge`. PreToolUse Bash boundary triggers tick
via the existing `loop_pre_bash_tick.sh` hook, so the nudge surfaces
at every Bash boundary the agent crosses.

**Suppress:** the existing `SQUAD_NO_CADENCE_NUDGES=1` env var
short-circuits both `timeBoxNudgeText` and `maybePrintTimeBoxNudge`.

**Tests:** `cmd/squad/cadence_nudge_test.go` adds 11 cases —
threshold edges (below 90m / at 90m without milestone / at 90m with
recent milestone / at 120m), env suppression (3 truthy values), DB
flow (fires once at 90m, fires once at 120m, recent milestone delays
fire then 90m fires after window expires, 120m skips unfired 90m
marker, no-claim no-op). Migration tests for the column shape and v12
bootstrap probe. All green.
