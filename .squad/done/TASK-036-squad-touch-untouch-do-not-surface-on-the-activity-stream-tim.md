---
id: TASK-036
title: squad touch / untouch don't surface on the activity-stream timeline
type: task
priority: P3
area: server
status: done
estimate: 1.5h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777256200
accepted_by: agent-401f
accepted_at: 1777256200
references:
  - cmd/squad/touch.go
  - internal/touch/touch.go
  - internal/server/activity_pump.go
  - internal/server/web/agent_timeline.js
relates-to:
  - TASK-028
  - TASK-029
blocked-by: []
---

## Problem

The activity-stream drawer is silent when an agent runs `squad touch`
or `squad untouch`. Everything else in the squad coordination
vocabulary already surfaces — the four-verb audit performed during
the TASK-029 walkthrough confirmed:

| verb | timeline surface |
|---|---|
| `squad release` | `item_changed` / `released` (claimsPump) |
| `squad blocked` | system message on global + item thread, plus claim release (messagesPump + claimsPump) |
| `squad progress` | `message` event from the dual-write to `messages` (CHORE-005 documents the duplication) |
| `squad touch` | **none** — `INSERT INTO touches` only, no chat post, no `agent_events` row |
| `squad untouch` | **none** — `DELETE FROM touches` only |

Touches are an explicit "I'm working on file X right now" signal —
exactly the data the drawer was built to surface. Today the drawer
goes blind during long-running edits where the agent declared touches
upfront but isn't running tool calls (e.g., a long Read or human
review pause).

## Context

- `cmd/squad/touch.go` runs `internal/touch/touch.go:Touch`
  (`INSERT INTO touches`) and `Untouch` (`DELETE FROM touches`).
  Neither posts to chat or emits an `agent_events` row.
- `internal/server/activity_pump.go` already polls three append-only
  tables (commits, attestations, agent_events) at 500 ms cadence;
  adding a fourth drain for `touches` is the smallest change.
- `internal/server/web/agent_timeline.js` `classify()` does not have
  a `touch` chip yet. CHIPS list is at `agent_timeline.js:5-15`.

## Acceptance criteria

- [ ] `internal/server/activity_pump.go` gains `drainTouches`
  paralleling `drainCommits` (snapshot-diff because `touches` rows
  can be deleted; the existing `agentsPump` does this for the
  `agents` table — match its shape, not the autoincrement-cursor
  shape `attestations`/`agent_events` use).
- [ ] Each `touches` row added emits a `Kind: "agent_activity"` SSE
  event with `payload.source = "touch"` and `payload.kind = "touch"`,
  carrying `agent_id`, `item_id`, `path`, and `ts` (use the existing
  `started_at` column).
- [ ] Each `touches` row removed emits a similar event with
  `payload.kind = "untouch"`.
- [ ] `agent_timeline.js` gains a new chip entry for `touch` in the
  `CHIPS` array, and `classify()` returns `['touch', null]` for
  rows where `kind === 'touch'` or `'untouch'`. `renderRow()` gains
  a `case 'touch':` / `case 'untouch':` arm with a badge + the
  `path`.
- [ ] `agent_detail.js` `toTimelineRow` already passes through
  `agent_activity` payloads unchanged — verify.
- [ ] Server-side tests in `internal/server/activity_pump_test.go`:
  one for touch insert, one for untouch delete, both asserting the
  SSE payload contents (mirror the shape of the existing 4 tests).
- [ ] Snapshot-diff at boot does NOT replay pre-existing touches as
  events (matches the no-replay invariant the other drains follow).
- [ ] `go test ./internal/server/... -count=1 -race` passes;
  `golangci-lint run` returns 0 issues.

## Notes

- **Snapshot-diff vs cursor.** `touches` rows are deleted on untouch
  (and on `agent_events`-style hygiene sweeps). A monotonic cursor
  would never see deletes; a snapshot diff per tick catches both
  directions. The `agents` pump pattern is the right template.
- **Don't merge with the existing `agent_events` writer path.** A
  touch is not a tool call — pumping it through `RecordEvent` would
  corrupt the table semantics the redaction config and Read-filter
  rely on. Keep touches as their own source.
- **Three siblings already surface and need no change** — verified
  during the TASK-029 walkthrough. They are listed in the table
  above so a future implementer doesn't accidentally double-pump
  the message-bus events.
- The chip default should be `on` (touches are sparse and signal-rich).
  Read-style auto-hide does not apply here.

## Resolution

(Filled in when status → done.)
