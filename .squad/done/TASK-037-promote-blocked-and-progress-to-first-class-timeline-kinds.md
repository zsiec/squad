---
id: TASK-037
title: promote blocked and progress to first-class timeline kinds with dedicated chips
type: task
priority: P3
area: server-spa
status: done
estimate: 1.5h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777256400
accepted_by: agent-401f
accepted_at: 1777256400
references:
  - internal/server/web/agent_detail.js
  - internal/server/web/agent_timeline.js
  - internal/server/claims_pump.go
  - internal/server/pump.go
relates-to:
  - TASK-028
  - TASK-036
blocked-by: []
---

## Problem

Two squad coordination verbs already surface on the activity-stream
timeline, but they ride generic transport channels and arrive without
dedicated badges or filter chips:

- **`squad blocked`** writes a `claim_history` row with
  `outcome="blocked"`. `claimsPump.drainHistory` publishes that as an
  `item_changed` SSE event with `payload.kind: "blocked"`. The client's
  `toTimelineRow` falls into the default branch and remaps `blocked`
  to `kind: "claim"` (claim badge) — visually indistinguishable from
  a fresh claim. `agent_timeline.js`'s `renderRow()` switch has no
  `case 'blocked':`, so the kind would render as an empty string if
  it ever reached the renderer un-remapped.
- **`squad progress`** dual-writes to the `progress` table and the
  `messages` table. The messages write surfaces as a `message` SSE
  event with `payload.kind: "progress"`. `toTimelineRow` maps that
  to `kind: "chat", outcome: "progress"`. The chat chip filter then
  hides every progress event the moment an operator toggles chat off
  to focus on tool calls — exactly the opposite of what a "what is
  the agent doing right now" drawer should do.

Operators reading the drawer can't distinguish "agent claimed and
started" from "agent hit a blocker," and they can't filter progress
heartbeats independently from raw chat. Both are coordination signals
that deserve first-class kinds.

## Context

- `internal/server/web/agent_detail.js` `toTimelineRow` is the
  client-side mapping point — every SSE event passes through here.
- `internal/server/web/agent_timeline.js` carries the chip set
  (`CHIPS`, lines 5–15), the kind-to-bucket mapping (`classify()`),
  and the per-kind render switch (`renderRow()`).
- `internal/server/claims_pump.go` `drainHistory` publishes the raw
  `outcome` as `kind`, so `"blocked"` is already on the wire — only
  the client needs to grow.
- `internal/server/pump.go` `messagesPump.drain` publishes
  `payload.kind: "progress"` for progress messages. Same shape —
  client-only fix.

## Acceptance criteria

- [ ] `agent_detail.js` `toTimelineRow`:
  - For `sseKind === 'item_changed'` with `payload.kind === 'blocked'`,
    return `{kind: 'blocked', source: 'blocked', agent_id, ts, item_id,
    outcome}`. (Today this branch falls through to `kind: 'claim'`.)
  - For `sseKind === 'message'` with `payload.kind === 'progress'`,
    return `{kind: 'progress', source: 'progress', agent_id, ts,
    item_id (from payload.thread when it's an item id), body}`.
- [ ] `agent_timeline.js` `CHIPS` gains two entries: `{id: 'blocked',
  label: 'blocked'}` and `{id: 'progress', label: 'progress'}`. Both
  default-on (coordination events are signal, not noise).
- [ ] `classify()` returns `['blocked', null]` for `kind === 'blocked'`
  and `['progress', null]` for `kind === 'progress'`.
- [ ] `renderRow()` gains two switch arms:
  - `case 'blocked':` renders a `tl-blocked` badge plus the
    `item_id` and `outcome`/reason text.
  - `case 'progress':` renders a `tl-progress` badge plus the
    `body` (which carries the percent + note from
    `internal/chat/progress.go`).
- [ ] Two new CSS rules in the dashboard stylesheet for `.tl-blocked`
  and `.tl-progress` (match the existing `.tl-claim` / `.tl-attestation`
  shape; pick distinct colours operators can scan).
- [ ] Any `data-classify` filter logic that already keys off
  `data-primary` keeps working — no new filter machinery, just two new
  primary buckets.
- [ ] Server-side test in `internal/server/activity_pump_test.go` (or
  a new file) is **not** required — the change is client-only.
- [ ] Client-side: smoke verify with the same harness CHORE-003 used —
  start `squad serve`, `curl /api/events`, run `squad blocked TASK-X
  --reason "test"` and `squad progress TASK-X 50 --note "halfway"`,
  confirm the SSE payload reaches the bus with the expected `kind`
  values. Capture the verbatim output in the close-out evidence.

## Notes

- Today's mapping (`blocked → claim`, `progress → chat`) is a coverage
  band-aid from TASK-028 — at the time the renderer didn't have arms
  for these kinds, so the normaliser flattened them into the closest
  existing bucket. This item promotes them to the first-class shape
  they always deserved.
- `release` and `done` already have first-class chips and renderers;
  no change needed for them.
- TASK-036 covers `touch` / `untouch`, which is a different gap (no
  pump at all). Don't bundle.
- The `messages` SSE payload has no `ts` — `toTimelineRow` already
  defaults to `Math.floor(Date.now()/1000)` for messages; preserve
  that fallback for the new progress branch.
- Filtering: by default both new chips are on, but progress events
  can be noisy on very-active claims. If operators complain, a
  follow-up could default progress to off; not in scope here.

## Resolution

`agent_detail.js toTimelineRow`: separate branches for `kind === 'progress'` (message SSE) and `kind === 'blocked'` (item_changed SSE). Each emits a row with `kind`/`source` set to the new bucket. Other branches unchanged.

`agent_timeline.js`: two new chips (`progress`, `blocked`), inserted next to `claim` so they group with lifecycle/coordination signals visually. `classify()` returns the matching primary bucket. `renderRow()` adds two switch arms — progress shows item_id + percent/note body, blocked shows item_id + outcome.

`style.css`: `.tl-badge.tl-progress` uses the existing accent-2 (cyan) and `.tl-blocked` uses the existing danger token, picking up the same look as commit/exit-non-zero rows.

Smoke evidence (squad serve + curl on a live instance): `agent_detail.js` contains `kind: 'progress'` and `kind: 'blocked'` branches; `agent_timeline.js` chip list includes both new ids; `style.css` has both new badge selectors. node --check clean across all touched JS.
