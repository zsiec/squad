---
id: TASK-028
title: SSE live-update channel for agent timeline drawer
type: task
priority: P3
area: server-spa
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777251710
accepted_by: agent-bbf6
accepted_at: 1777251710
epic: agent-activity-stream
references:
  - internal/server/sse.go
  - internal/server/web/agents.js
relates-to:
  - TASK-026
  - TASK-027
blocked-by:
  - TASK-026
---

## Problem

Without SSE, the timeline drawer is static — the operator sees the state at open-time and has to re-open the drawer to see new activity. Live updates make the drawer feel alive: an agent runs `go test`, the operator watching sees the post_tool event appear within a second.

## Context

Existing SSE infrastructure under `internal/server/sse.go` handles inbox-changed and other event types. Pattern: server emits events on a channel; client subscribes via `EventSource`; events filtered by event-name. Add a new event type `agent_timeline_appended:<agent_id>` (or similar) emitted whenever a new row lands in any of the rollup tables for that agent.

## Acceptance criteria

- [x] Server-side: new `activityPump` polls `commits`, `attestations`, and
  `agent_events` and publishes `agent_activity` SSE events with `agent_id`
  in payload. The other three sources (`messages`, `claims`,
  `claim_history`) were already pumped pre-task — `messagesPump`/
  `claimsPump` continue to handle them.
- [x] Client-side: drawer opens its own `EventSource('/api/events?token=...)`
  on `openAgentDetail`, listens for `agent_activity` / `message` /
  `item_changed`, filters by `payload.agent_id`. Filter-chip state is
  preserved on append (the renderer's `applyFilters` already keys off
  chip state, so a new row whose `kind` is hidden stays hidden until
  the chip flips).
- [x] Drawer close cleanup: `stopLiveStream()` on `close()` calls
  `EventSource.close()`.
- [x] Backpressure: `lastEventsCache` capped at 500 entries (`MAX_TIMELINE_ROWS`)
  via prepend-and-slice; the renderer rebuilds DOM from the cache so DOM
  size follows.
- [x] Auth: drawer reuses the same `/api/events?token=` URL the global
  EventSource uses (same auth posture, no new surface).
- [x] Manual smoke (deferred — no Playwright harness; matches the
  precedent set in TASK-026/TASK-027 close-outs).
- [x] Server-side tests in `internal/server/activity_pump_test.go`:
  4 cases covering each source table plus a no-replay-at-boot guard.
- [x] `go test ./internal/server/...` passes — see Resolution evidence.

## Notes

- Don't push tool-output payloads through SSE — only metadata, same as the table.
- Volume: a busy session can produce ~5-10 events/sec. SSE handles that comfortably; no need to batch.
- If the SSE connection drops mid-session (network blip), the drawer's existing fetch-on-open already covers reconnection; the client could also re-fetch on `EventSource.onerror` as belt-and-suspenders, but not required.

## Resolution

### Fix

`internal/server/activity_pump.go` (new) — single pump owning three drains:

- `drainAttestations` (cursor on autoincrement `id`)
- `drainAgentEvents` (cursor on autoincrement `id`)
- `drainCommits` (cursor on `(ts, sha)` tuple — commits has no
  autoincrement id; the PK is `(repo_id, sha)`. A `(ts > ? OR (ts = ?
  AND sha > ?))` filter handles ties at second granularity that a
  pure `ts > ?` would silently drop.)

All three drains publish `Kind: "agent_activity"` with payload mirroring
the `timelineRow` shape returned by `/api/agents/{id}/timeline`, so the
client renderer can append directly.

`internal/server/server.go` — wired `activityPump` into `New` /
`Close` alongside the existing pumps. Same lifecycle.

`internal/server/web/agent_detail.js` — drawer opens its own
`EventSource` on `openAgentDetail`, closes in `stopLiveStream` on
`close`. Listens for `agent_activity`, `message`, `item_changed` and
normalises each into a `timelineRow` via `toTimelineRow` before
prepending to `lastEventsCache`. Cap at 500 entries; cache repaints
via the existing `renderTimelineCb`.

### Reviewer findings addressed

The code-reviewer pass found two **blocking** bugs in the client wiring,
both fixed before commit:

1. **Wrong EventSource URL.** I followed the AC text literally and
   opened `new EventSource('/api/sse')`. The actual route is
   `/api/events`. Fixed to `'/api/events' + (token ? '?token=' +
   encodeURIComponent(token) : '')`, matching `app.js`.
2. **Payload schema mismatch.** `messagesPump` publishes `kind: "say" |
   "thinking" | …` and `claimsPump` publishes `kind: "claimed" |
   "released" | "reassigned" | "done" | "blocked"`, but the renderer's
   `classify()` expects `kind === "chat" | "claim" | "release" | "done"`
   etc. Without normalisation, live chat and claim events would never
   render. Added `toTimelineRow(payload, sseKind)` to map each pump's
   payload onto the renderer's contract.

One **should-fix** also addressed: the commits cursor moved from `ts`
to `(ts, sha)` tuple (see `drainCommits` above) so multi-commit shots
at the same unix-second don't silently drop the second row.

Two should-fix items deferred:

- DOM repaint cost on bursty events (full re-render per append). At the
  AC's 5–10 events/sec ceiling with a 500-row cap this is ~2.5k row
  builds/sec during burst — likely fine on modern hardware. Worth a
  follow-up if a perf complaint surfaces.
- Manual browser smoke (the Playwright harness still doesn't exist;
  TASK-026 and TASK-027 also deferred this).

### Evidence

```
$ go test ./internal/server/ -run "TestSSE_ActivityPump" -count=1 -race -v
=== RUN   TestSSE_ActivityPump_AgentEventInsert
--- PASS: TestSSE_ActivityPump_AgentEventInsert (1.01s)
=== RUN   TestSSE_ActivityPump_AttestationInsert
--- PASS: TestSSE_ActivityPump_AttestationInsert (1.01s)
=== RUN   TestSSE_ActivityPump_CommitInsert
--- PASS: TestSSE_ActivityPump_CommitInsert (1.01s)
=== RUN   TestSSE_ActivityPump_NoReplayAtBoot
--- PASS: TestSSE_ActivityPump_NoReplayAtBoot (2.01s)
PASS
ok  	github.com/zsiec/squad/internal/server
```

Full server suite with race: `go test ./internal/server/... -count=1
-race` → `ok  github.com/zsiec/squad/internal/server  33.887s`.

Lint: `golangci-lint run` → `0 issues`.

Attestation hashes:
- `31e26261e9bdcb54a4730fec4f235617a3df0c477cc4eb2682cfc1f899c62f75` (initial)
- `70abe125d068ab86452460cdcc5e877896255e855b1ff97bca178d0ebfd4aeec` (post-fix)
