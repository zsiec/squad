---
id: TASK-028
title: SSE live-update channel for agent timeline drawer
type: task
priority: P3
area: server-spa
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: 2026-04-27
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

- [ ] Server-side: when `messages`, `claims`, `claim_history`, `commits`, `attestations`, or `agent_events` insert a row, an SSE event is published with the new row's payload + the agent_id. Use the existing SSE plumbing — find where `inbox_changed` is emitted as the pattern.
- [ ] Client-side: when the drawer is open for `agent_id=X`, subscribe to `EventSource('/api/sse')` filtering for events with `agent_id == X`. Append new items to the timeline as they arrive (respecting the active filter chips — if `Read` is hidden, new Read events still arrive but are not rendered until the toggle flips).
- [ ] Drawer close cleanup: `EventSource.close()` is called when the drawer closes. Verify in DevTools network panel that the SSE connection drops.
- [ ] Backpressure: if the timeline already has 500+ rows, oldest visible rows scroll out (DOM cap at 500 to prevent unbounded memory).
- [ ] Auth: SSE inherits the existing `/api/sse` auth posture — don't add a new auth surface.
- [ ] Manual smoke:
  1. Open the drawer for `agent-X` in tab A
  2. From tab B (or terminal), do something as `agent-X` — `squad fyi "test"` or run a Bash command in a session
  3. Within ~1s, the new entry appears in tab A's drawer without refresh
  4. Close the drawer in tab A; verify the SSE connection terminates in DevTools
- [ ] Server-side tests in `internal/server/sse_test.go`: SSE event published on insert, payload contains the new row.
- [ ] `go test ./internal/server/...` passes.

## Notes

- Don't push tool-output payloads through SSE — only metadata, same as the table.
- Volume: a busy session can produce ~5-10 events/sec. SSE handles that comfortably; no need to batch.
- If the SSE connection drops mid-session (network blip), the drawer's existing fetch-on-open already covers reconnection; the client could also re-fetch on `EventSource.onerror` as belt-and-suspenders, but not required.

## Resolution

(Filled in when status → done.)
